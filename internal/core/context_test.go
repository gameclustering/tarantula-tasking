package core

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func callback(ctx context.Context, c <-chan string) {
	for {
		select {
		case dt := <-ctx.Done():
			fmt.Printf("done %v\n", dt)
			//for s := range c {
			//fmt.Printf("result after done :%s\n", s)
			//}
			return
		case s := <-c:
			fmt.Printf("result :%s\n", s)
		default:
			time.Sleep(100 * time.Millisecond)
			//fmt.Println("waiting ...")
		}
	}
}

func task(ctx context.Context, id int) {
	select {
	case <-ctx.Done(): // “Abort” signal
		fmt.Printf("Task %d early bailed: %v\n", id, ctx.Err())
		return
	default:
	}
	ch := make(chan int, 1)
	go func() {
		ch <- id + 10
	}()
	select {
	case r := <-ch: // Fake work
		fmt.Printf("Task %d done!\n", r)
	case <-ctx.Done(): // “Abort” signal
		fmt.Printf("Task %d bailed: %v\n", id, ctx.Err())
	}
}

func TestCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	c := make(chan string, 2)
	defer func() {
		fmt.Println("clean up")
		if err := ctx.Err(); err != nil {
			fmt.Printf("err %s\n", err.Error())
		}
		close(c)
		cancel()
	}()
	go callback(ctx, c)
	c <- "a"
	time.Sleep(1 * time.Second)
	c <- "b"
	time.Sleep(1 * time.Second)
	c <- "c"
	cancel() //early cancel
	time.Sleep(1 * time.Second)
	c <- "d"
	time.Sleep(1 * time.Second)
}

func TestTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	c := make(chan string, 2)
	defer func() {
		fmt.Println("timeout up")
		if err := ctx.Err(); err != nil {
			fmt.Printf("err %s\n", err.Error())
		}
		close(c)
		cancel()
	}()
	go callback(ctx, c)
	//for {
	c <- "ta"
	time.Sleep(1 * time.Second)
	c <- "tb"
	time.Sleep(1 * time.Second)
	c <- "tc"
	//cancel() //early cancel
	time.Sleep(1 * time.Second)
	c <- "td"
	time.Sleep(1 * time.Second)
	//}
}

func TestTask(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	for i := 1; i <= 3; i++ {
		go task(ctx, i)
	}
	//cancel()
	time.Sleep(1 * time.Second)
	//cancel()
	time.Sleep(3 * time.Second)
}
