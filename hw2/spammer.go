package main

import (
	"fmt"
	"sync"
)

func RunPipeline(cmds ...cmd) {

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
}

func CheckSpam(in, out chan interface{}) {
	// in - MsgID
	// out - MsgData
}

func CombineResults(in, out chan interface{}) {
	// in - MsgData
	// out - string
}
