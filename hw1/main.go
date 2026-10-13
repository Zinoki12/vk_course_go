package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Item struct {
	Name      string
	Wearable  bool
	Container bool
}

type Place struct {
	Name  string
	Items []*Item
}

func (p *Place) remove(i int) *Item {
	item := p.Items[i]
	p.Items = append(p.Items[:i], p.Items[i+1:]...)
	return item
}

type Applicable interface {
	Apply(item string) (string, bool)
}

type Door struct {
	Name string
	Key  string
	Open bool
}

func (d *Door) Apply(item string) (string, bool) {
	if item != d.Key {
		return "", false
	}
	d.Open = true
	return d.Name + " открыта", true
}

type Exit struct {
	Name string
	To   *Room
	Door *Door
}

type Room struct {
	Name      string
	EnterText string
	EmptyText string
	Places    []*Place
	Exits     []*Exit
	Targets   map[string]Applicable
	Look      func(r *Room) string
}

func (r *Room) itemsText() string {
	parts := make([]string, 0, len(r.Places))
	for _, p := range r.Places {
		if len(p.Items) == 0 {
			continue
		}
		names := make([]string, 0, len(p.Items))
		for _, item := range p.Items {
			names = append(names, item.Name)
		}
		parts = append(parts, p.Name+": "+strings.Join(names, ", "))
	}
	return strings.Join(parts, ", ")
}

func (r *Room) exitsText() string {
	names := make([]string, 0, len(r.Exits))
	for _, e := range r.Exits {
		names = append(names, e.Name)
	}
	return "можно пройти - " + strings.Join(names, ", ")
}

func (r *Room) look() string {
	if r.Look != nil {
		return r.Look(r) + ". " + r.exitsText()
	}
	if items := r.itemsText(); items != "" {
		return items + ". " + r.exitsText()
	}
	return r.EmptyText + ". " + r.exitsText()
}

func (r *Room) enter() string {
	return r.EnterText + ". " + r.exitsText()
}

func (r *Room) exit(name string) *Exit {
	for _, e := range r.Exits {
		if e.Name == name {
			return e
		}
	}
	return nil
}

func (r *Room) findItem(name string) (*Place, int) {
	for _, p := range r.Places {
		for i, item := range p.Items {
			if item.Name == name {
				return p, i
			}
		}
	}
	return nil, -1
}

type Player struct {
	Room      *Room
	Worn      []*Item
	Inventory []*Item
}

func (p *Player) canCarry() bool {
	for _, item := range p.Worn {
		if item.Container {
			return true
		}
	}
	return false
}

func (p *Player) wears(name string) bool {
	for _, item := range p.Worn {
		if item.Name == name {
			return true
		}
	}
	return false
}

func (p *Player) has(name string) bool {
	for _, item := range p.Inventory {
		if item.Name == name {
			return true
		}
	}
	return p.wears(name)
}

type Task struct {
	Text string
	Done func() bool
}

func goal() string {
	left := make([]string, 0, len(tasks))
	for _, t := range tasks {
		if !t.Done() {
			left = append(left, t.Text)
		}
	}
	return "надо " + strings.Join(left, " и ")
}

type command struct {
	args int
	run  func(args []string) string
}

var commands = map[string]command{
	"осмотреться": {0, cmdLook},
	"идти":        {1, cmdGo},
	"взять":       {1, cmdTake},
	"надеть":      {1, cmdWear},
	"применить":   {2, cmdApply},
}

var (
	player *Player
	tasks  []*Task
)

func cmdLook(_ []string) string {
	return player.Room.look()
}

func cmdGo(args []string) string {
	e := player.Room.exit(args[0])
	if e == nil {
		return "нет пути в " + args[0]
	}
	if e.Door != nil && !e.Door.Open {
		return e.Door.Name + " закрыта"
	}
	player.Room = e.To
	return e.To.enter()
}

func cmdTake(args []string) string {
	place, i := player.Room.findItem(args[0])
	if place == nil {
		return "нет такого"
	}
	if !player.canCarry() {
		return "некуда класть"
	}
	player.Inventory = append(player.Inventory, place.remove(i))
	return "предмет добавлен в инвентарь: " + args[0]
}

func cmdWear(args []string) string {
	place, i := player.Room.findItem(args[0])
	if place == nil {
		return "нет такого"
	}
	if !place.Items[i].Wearable {
		return "нельзя надеть: " + args[0]
	}
	player.Worn = append(player.Worn, place.remove(i))
	return "вы надели: " + args[0]
}

func cmdApply(args []string) string {
	item, target := args[0], args[1]
	if !player.has(item) {
		return "нет предмета в инвентаре - " + item
	}
	t, ok := player.Room.Targets[target]
	if !ok {
		return "не к чему применить"
	}
	answer, ok := t.Apply(item)
	if !ok {
		return "не к чему применить"
	}
	return answer
}

func main() {
	initGame()
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		fmt.Println(handleCommand(scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка чтения:", err)
	}
}

func initGame() {
	kitchen := &Room{
		Name:      "кухня",
		EnterText: "кухня, ничего интересного",
		Places: []*Place{
			{Name: "на столе", Items: []*Item{{Name: "чай"}}},
		},
		Look: func(r *Room) string {
			parts := []string{"ты находишься на кухне"}
			if items := r.itemsText(); items != "" {
				parts = append(parts, items)
			}
			return strings.Join(append(parts, goal()), ", ")
		},
	}
	corridor := &Room{
		Name:      "коридор",
		EnterText: "ничего интересного",
		EmptyText: "ничего интересного",
	}
	bedroom := &Room{
		Name:      "комната",
		EnterText: "ты в своей комнате",
		EmptyText: "пустая комната",
		Places: []*Place{
			{Name: "на столе", Items: []*Item{{Name: "ключи"}, {Name: "конспекты"}}},
			{Name: "на стуле", Items: []*Item{{Name: "рюкзак", Wearable: true, Container: true}}},
		},
	}
	street := &Room{
		Name:      "улица",
		EnterText: "на улице весна",
		EmptyText: "на улице весна",
	}

	door := &Door{Name: "дверь", Key: "ключи"}
	corridor.Targets = map[string]Applicable{door.Name: door}

	kitchen.Exits = []*Exit{{Name: "коридор", To: corridor}}
	corridor.Exits = []*Exit{
		{Name: "кухня", To: kitchen},
		{Name: "комната", To: bedroom},
		{Name: "улица", To: street, Door: door},
	}
	bedroom.Exits = []*Exit{{Name: "коридор", To: corridor}}
	street.Exits = []*Exit{{Name: "домой", To: corridor, Door: door}}

	player = &Player{Room: kitchen}
	tasks = []*Task{
		{Text: "собрать рюкзак", Done: func() bool { return player.wears("рюкзак") }},
		{Text: "идти в универ", Done: func() bool { return false }}, // универа в мире пока нет
	}
}

func handleCommand(command string) string {
	words := strings.Fields(command)
	if len(words) == 0 {
		return "неизвестная команда"
	}
	cmd, ok := commands[words[0]]
	if !ok || len(words)-1 != cmd.args {
		return "неизвестная команда"
	}
	return cmd.run(words[1:])
}
