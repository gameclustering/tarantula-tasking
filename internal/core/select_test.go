package core

import (
	"fmt"
	"testing"
	"time"
)

func TestSelectOne(t *testing.T) {
	ch1 := make(chan int, 10)
	ch2 := make(chan int, 10)

	go func() {
		for i := range 5 {
			ch1 <- i + 1
		}
	}()
	go func() {
		for i := range 5 {
			ch2 <- i + 100
		}
	}()

	select {
	case x := <-ch1:
		fmt.Printf("xfrom ch1 >> %d\n", x)
	case y := <-ch2:
		fmt.Printf("xfrom ch2 >> %d\n", y)
	}
}

func TestSelectTimer(t *testing.T) {
	ch1 := make(chan int, 10)
	ch2 := make(chan int, 10)

	go func() {
		time.Sleep(1 * time.Second)
		for i := range 5 {
			ch1 <- i + 1
		}
		close(ch1)
	}()
	go func() {
		time.Sleep(1 * time.Second)
		for i := range 5 {
			ch2 <- i + 100
		}
		close(ch2)
	}()
	tc := time.NewTimer(500 * time.Millisecond)
	defer tc.Stop()
	select {
	case x := <-ch1:
		fmt.Printf("xfrom ch1 >> %d\n", x)
	case y := <-ch2:
		fmt.Printf("xfrom ch2 >> %d\n", y)
	case <-tc.C:
		fmt.Println("timeout")
	}
}

func TestSelectBreak(t *testing.T) {
	ch1 := make(chan int, 10)
	ch2 := make(chan int, 10)

	go func() {
		for i := range 5 {
			ch1 <- i + 1
		}
		close(ch1)
	}()
	go func() {
		for i := range 5 {
			ch2 <- i + 100
		}
		close(ch2)
	}()
loop:
	for {
		select {
		case x, ok := <-ch1:
			if !ok {
				break loop
			}
			fmt.Printf("from ch1 >> %d\n", x)

		case y, ok := <-ch2:
			if !ok {
				break loop
			}
			fmt.Printf("from ch2 >> %d\n", y)
		}
	}
}

func TestSelectSleep(t *testing.T) {
	ch1 := make(chan int, 10)
	ch2 := make(chan int, 10)

	go func() {
		for i := range 5 {
			ch1 <- i + 1
			time.Sleep(500 * time.Millisecond)
		}
		time.Sleep(1000 * time.Millisecond)
		close(ch1)
	}()
	go func() {
		for i := range 5 {
			ch2 <- i + 101
			time.Sleep(300 * time.Millisecond)
		}
		time.Sleep(1000 * time.Millisecond)
		close(ch2)
	}()
loop:
	for {
		select {
		case x, ok := <-ch1:
			if !ok {
				break loop
			}
			fmt.Printf("S from ch1 >> %d\n", x)

		case y, ok := <-ch2:
			if !ok {
				break loop
			}
			fmt.Printf("S from ch2 >> %d\n", y)
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}
