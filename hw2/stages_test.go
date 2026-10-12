package main

// Тесты для отладки каждого этапа по отдельности, без RunPipeline.
// Запуск одной группы:
//   go test -v -race -run TestSelectUsers
//   go test -v -race -run TestSelectMessages
//   go test -v -race -run TestCheckSpam
//   go test -v -race -run TestCombineResults
//   go test -v -race -run TestRunPipeline

import (
	"fmt"
	"hash/crc64"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------- помощники ----------

// runStage запускает один этап напрямую: кладёт input в in, закрывает in,
// после завершения этапа закрывает out и возвращает всё, что пришло из out.
// Если этап не завершился за timeout - тест падает.
func runStage(t *testing.T, stage cmd, input []interface{}, timeout time.Duration) ([]interface{}, time.Duration) {
	t.Helper()

	in := make(chan interface{}, len(input))
	for _, v := range input {
		in <- v
	}
	close(in)

	out := make(chan interface{})
	start := time.Now()

	done := make(chan struct{})
	go func() {
		defer close(done)
		stage(in, out)
	}()
	go func() {
		<-done
		defer func() { _ = recover() }() // на случай, если этап сам закрыл out
		close(out)
	}()

	var res []interface{}
	deadline := time.After(timeout)
	for {
		select {
		case v, ok := <-out:
			if !ok {
				return res, time.Since(start)
			}
			res = append(res, v)
		case <-deadline:
			t.Fatalf("этап не завершился за %s (получено значений: %d)", timeout, len(res))
		}
	}
}

// startStage запускает этап с открытым входным каналом - для проверки,
// что данные идут дальше сразу, не дожидаясь конца входа.
func startStage(stage cmd) (in, out chan interface{}, done chan struct{}) {
	in = make(chan interface{}, 100)
	out = make(chan interface{}, 100)
	done = make(chan struct{})
	go func() {
		defer close(done)
		stage(in, out)
	}()
	return in, out, done
}

func waitValue(t *testing.T, out chan interface{}, timeout time.Duration) interface{} {
	t.Helper()
	select {
	case v := <-out:
		return v
	case <-time.After(timeout):
		t.Fatalf("за %s из out ничего не пришло, хотя вход ещё не закрыт", timeout)
		return nil
	}
}

func waitDone(t *testing.T, done chan struct{}, timeout time.Duration) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("этап не завершился за %s после закрытия входа", timeout)
	}
}

func toInterfaces[T any](xs []T) []interface{} {
	res := make([]interface{}, 0, len(xs))
	for _, x := range xs {
		res = append(res, x)
	}
	return res
}

// emailUser считает User так же, как GetUser, но без задержки и без алиасов
func emailUser(email string) User {
	return User{
		ID:    crc64.Checksum([]byte(email), crc64.MakeTable(crc64.ISO)),
		Email: email,
	}
}

func usersN(n int) []User {
	users := make([]User, 0, n)
	for i := 0; i < n; i++ {
		users = append(users, emailUser(fmt.Sprintf("user%d@mail.ru", i)))
	}
	return users
}

// expectedMessages повторяет генерацию писем из GetMessages
func expectedMessages(users ...User) []MsgID {
	var res []MsgID
	for _, u := range users {
		r := rand.New(rand.NewSource(int64(u.ID))) //nolint: gosec
		n := r.Intn(10)
		for i := 0; i <= n; i++ {
			res = append(res, MsgID(r.Uint64()))
		}
	}
	return res
}

// expectedSpam повторяет ответ HasSpam
func expectedSpam(id MsgID) bool {
	r := rand.New(rand.NewSource(int64(id))) //nolint: gosec
	return r.Intn(2) == 1
}

var totalEmails = []string{
	"harry.dubois@mail.ru",
	"k.kitsuragi@mail.ru",
	"d.vader@mail.ru",
	"noname@mail.ru",
	"e.musk@mail.ru",
	"spiderman@mail.ru",
	"red_prince@mail.ru",
	"tomasangelo@mail.ru",
	"batman@mail.ru",
	"bruce.wayne@mail.ru",
}

// ---------- SelectUsers ----------

func TestSelectUsers_One(t *testing.T) {
	stat = Stat{}
	res, _ := runStage(t, SelectUsers, toInterfaces([]string{"harry.dubois@mail.ru"}), 3*time.Second)

	assert.Equal(t, []interface{}{emailUser("harry.dubois@mail.ru")}, res)
	assert.Equal(t, uint32(1), stat.RunGetUser, "сколько раз вызван GetUser")
}

func TestSelectUsers_Empty(t *testing.T) {
	stat = Stat{}
	res, _ := runStage(t, SelectUsers, nil, time.Second)

	assert.Empty(t, res)
	assert.Equal(t, uint32(0), stat.RunGetUser)
}

func TestSelectUsers_Alias(t *testing.T) {
	stat = Stat{}
	input := toInterfaces([]string{"batman@mail.ru", "bruce.wayne@mail.ru"})
	res, _ := runStage(t, SelectUsers, input, 3*time.Second)

	assert.Equal(t, []interface{}{emailUser("bruce.wayne@mail.ru")}, res,
		"batman и bruce.wayne - один и тот же юзер")
	assert.Equal(t, uint32(2), stat.RunGetUser)
}

func TestSelectUsers_Total(t *testing.T) {
	stat = Stat{}
	res, elapsed := runStage(t, SelectUsers, toInterfaces(totalEmails), 5*time.Second)

	expected := []interface{}{
		emailUser("harry.dubois@mail.ru"),
		emailUser("k.kitsuragi@mail.ru"),
		emailUser("d.vader@mail.ru"),
		emailUser("noname@mail.ru"),
		emailUser("e.musk@mail.ru"),
		emailUser("peter.parker@mail.ru"),
		emailUser("red_prince@mail.ru"),
		emailUser("tomasangelo@mail.ru"),
		emailUser("bruce.wayne@mail.ru"),
	}
	assert.ElementsMatch(t, expected, res, "порядок не важен, важен состав")
	assert.Equal(t, uint32(10), stat.RunGetUser)
	assert.Less(t, elapsed, 1500*time.Millisecond, "10 вызовов по 1с должны идти параллельно")
}

func TestSelectUsers_ManyDuplicates(t *testing.T) {
	stat = Stat{}
	emails := make([]string, 300)
	for i := range emails {
		emails[i] = fmt.Sprintf("user%d@mail.ru", i/10) // каждый email повторяется 10 раз
	}
	res, elapsed := runStage(t, SelectUsers, toInterfaces(emails), 5*time.Second)

	assert.ElementsMatch(t, toInterfaces(usersN(30)), res)
	assert.Less(t, elapsed, 2*time.Second)
}

func TestSelectUsers_Streaming(t *testing.T) {
	stat = Stat{}
	in, out, done := startStage(SelectUsers)

	in <- "harry.dubois@mail.ru"
	v := waitValue(t, out, 1500*time.Millisecond)
	assert.Equal(t, emailUser("harry.dubois@mail.ru"), v)

	close(in)
	waitDone(t, done, 2*time.Second)
}

// ---------- SelectMessages ----------

func TestSelectMessages_One(t *testing.T) {
	stat = Stat{}
	u := emailUser("harry.dubois@mail.ru")
	res, _ := runStage(t, SelectMessages, []interface{}{u}, 3*time.Second)

	assert.ElementsMatch(t, toInterfaces(expectedMessages(u)), res)
	assert.Equal(t, uint32(1), stat.RunGetMessages)
	assert.Equal(t, uint32(1), stat.GetMessagesTotalUsers)
}

func TestSelectMessages_Empty(t *testing.T) {
	stat = Stat{}
	res, _ := runStage(t, SelectMessages, nil, time.Second)

	assert.Empty(t, res)
	assert.Equal(t, uint32(0), stat.RunGetMessages)
}

func TestSelectMessages_Batches(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 9} {
		t.Run(fmt.Sprintf("users=%d", n), func(t *testing.T) {
			stat = Stat{}
			users := usersN(n)
			res, elapsed := runStage(t, SelectMessages, toInterfaces(users), 5*time.Second)

			for _, v := range res {
				assert.IsType(t, MsgID(0), v, "в out должны лежать MsgID")
			}
			assert.ElementsMatch(t, toInterfaces(expectedMessages(users...)), res)
			assert.Equal(t, uint32((n+1)/2), stat.RunGetMessages, "сколько раз вызван GetMessages")
			assert.Equal(t, uint32(n), stat.GetMessagesTotalUsers, "сколько юзеров всего передано в GetMessages")
			assert.Equal(t, uint32(0), stat.ErrorGetMessage)
			assert.Less(t, elapsed, 1500*time.Millisecond, "вызовы GetMessages должны идти параллельно")
		})
	}
}

func TestSelectMessages_Streaming(t *testing.T) {
	stat = Stat{}
	in, out, done := startStage(SelectMessages)

	in <- emailUser("harry.dubois@mail.ru")
	in <- emailUser("k.kitsuragi@mail.ru")
	v := waitValue(t, out, 1500*time.Millisecond)
	assert.IsType(t, MsgID(0), v)

	close(in)
	waitDone(t, done, 2*time.Second)
}

// ---------- CheckSpam ----------

func TestCheckSpam_One(t *testing.T) {
	stat = Stat{}
	id := MsgID(12386730660396758454)
	res, _ := runStage(t, CheckSpam, []interface{}{id}, time.Second)

	assert.Equal(t, []interface{}{MsgData{ID: id, HasSpam: expectedSpam(id)}}, res)
	assert.Equal(t, uint32(1), stat.RunHasSpam)
}

func TestCheckSpam_Empty(t *testing.T) {
	stat = Stat{}
	res, _ := runStage(t, CheckSpam, nil, time.Second)

	assert.Empty(t, res)
	assert.Equal(t, uint32(0), stat.RunHasSpam)
}

func TestCheckSpam_Many(t *testing.T) {
	stat = Stat{}
	ids := make([]MsgID, 42)
	expected := make([]MsgData, 42)
	for i := range ids {
		ids[i] = MsgID(1000 + i)
		expected[i] = MsgData{ID: ids[i], HasSpam: expectedSpam(ids[i])}
	}
	res, elapsed := runStage(t, CheckSpam, toInterfaces(ids), 6*time.Second)

	assert.ElementsMatch(t, toInterfaces(expected), res)
	assert.Equal(t, uint32(42), stat.RunHasSpam)
	assert.Equal(t, uint32(0), stat.ErrorHasSpam, "антиспам вернул ошибку - слишком много параллельных запросов")
	assert.Less(t, elapsed, 1500*time.Millisecond, "42 запроса по 100мс должны идти параллельно")
}

func TestCheckSpam_Streaming(t *testing.T) {
	stat = Stat{}
	in, out, done := startStage(CheckSpam)

	in <- MsgID(42)
	v := waitValue(t, out, 500*time.Millisecond)
	assert.Equal(t, MsgData{ID: 42, HasSpam: expectedSpam(42)}, v)

	close(in)
	waitDone(t, done, time.Second)
}

// ---------- CombineResults ----------

func TestCombineResults_Sort(t *testing.T) {
	input := []MsgData{
		{ID: 5, HasSpam: false},
		{ID: 10, HasSpam: true},
		{ID: 1, HasSpam: false},
		{ID: 9, HasSpam: true},
		{ID: 100, HasSpam: false},
	}
	res, _ := runStage(t, CombineResults, toInterfaces(input), time.Second)

	expected := []interface{}{
		"true 9",
		"true 10",
		"false 1",
		"false 5",
		"false 100",
	}
	assert.Equal(t, expected, res)
}

func TestCombineResults_Empty(t *testing.T) {
	res, _ := runStage(t, CombineResults, nil, time.Second)
	assert.Empty(t, res)
}

// ---------- RunPipeline ----------

func runPipelineWithTimeout(t *testing.T, timeout time.Duration, cmds ...cmd) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		RunPipeline(cmds...)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("RunPipeline не завершился за %s", timeout)
	}
}

func TestRunPipeline_Simple(t *testing.T) {
	var (
		mu  sync.Mutex
		res []int
	)
	runPipelineWithTimeout(t, time.Second,
		func(in, out chan interface{}) {
			for i := 1; i <= 5; i++ {
				out <- i
			}
		},
		func(in, out chan interface{}) {
			for v := range in {
				out <- v.(int) * 2
			}
		},
		func(in, out chan interface{}) {
			for v := range in {
				mu.Lock()
				res = append(res, v.(int))
				mu.Unlock()
			}
		},
	)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int{2, 4, 6, 8, 10}, res)
}

func TestRunPipeline_OneStage(t *testing.T) {
	called := false
	runPipelineWithTimeout(t, time.Second, func(in, out chan interface{}) {
		called = true
	})
	assert.True(t, called)
}
