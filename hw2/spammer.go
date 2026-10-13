package main

import (
	"cmp"
	"fmt"
	"slices"
	"sync"
)

func RunPipeline(cmds ...cmd) {
	in := make(chan interface{})
	close(in)
	var wg sync.WaitGroup
	for _, cmd := range cmds {
		out := make(chan interface{})
		wg.Add(1)
		go func(in, out chan interface{}) {
			defer wg.Done()
			cmd(in, out)
			close(out)
		}(in, out)
		in = out
	}
	wg.Wait()
}

func SelectUsers(in, out chan interface{}) {
	// 	in - string
	// 	out - User
	set := make(map[uint64]struct{})
	var mu sync.Mutex
	var wg sync.WaitGroup

	for v := range in {
		wg.Add(1)
		go func(v interface{}) {
			defer wg.Done()
			email, ok := v.(string)
			if !ok {
				fmt.Println("Not a string!")
				return
			}
			user := GetUser(email)

			mu.Lock()
			_, ok = set[user.ID]
			if ok {
				mu.Unlock()
				return
			}
			set[user.ID] = struct{}{}
			mu.Unlock()
			out <- user
		}(v)
	}
	wg.Wait()
}

func SelectMessages(in, out chan interface{}) {
	// 	in - User
	// 	out - MsgID
	batch := make([]User, 0, 2)
	var wg sync.WaitGroup
	for v := range in {
		el, ok := v.(User)
		if !ok {
			continue
		}
		batch = append(batch, el)

		if len(batch) == 2 {
			wg.Add(1)
			go func(batch []User) {
				defer wg.Done()

				ids, err := GetMessages(batch...)
				if err != nil {
					return
				}
				for _, id := range ids {
					out <- id
				}
			}(batch)
			batch = nil
		}

	}

	if len(batch) != 0 {
		wg.Add(1)
		go func(batch []User) {
			defer wg.Done()
			ids, err := GetMessages(batch...)
			if err != nil {
				return
			}
			for _, id := range ids {
				out <- id
			}
		}(batch)
	}

	wg.Wait()
}

func CheckSpam(in, out chan interface{}) {
	sem := make(chan struct{}, HasSpamMaxAsyncRequests)
	var wg sync.WaitGroup
	for v := range in {
		val, ok := v.(MsgID)
		if !ok {
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(val MsgID) {
			defer wg.Done()
			res, err := HasSpam(val)
			<-sem
			if err != nil {
				return
			}
			data := MsgData{ID: val, HasSpam: res}
			out <- data
		}(val)

	}
	wg.Wait()
}

func CombineResults(in, out chan interface{}) {
	// in - MsgData
	// out - string
	data := make([]MsgData, 0, 128)
	for v := range in {
		val, ok := v.(MsgData)
		if !ok {
			continue
		}
		data = append(data, val)
	}
	slices.SortFunc(data, func(a, b MsgData) int {
		if a.HasSpam != b.HasSpam {
			if a.HasSpam {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.ID, b.ID)
	})
	for _, el := range data {
		str := fmt.Sprintf("%t %d", el.HasSpam, el.ID)
		out <- str
	}
}
