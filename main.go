package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Debouncer[T any] struct {
	messages []T
	interval time.Duration
	timer    *time.Timer
	mu       sync.Mutex
}

func NewDebouncer[T any](interval time.Duration) *Debouncer[T] {
	return &Debouncer[T]{
		interval: interval,
	}
}

func (d *Debouncer[T]) Debounce(msg T, wg *sync.WaitGroup, f func([]T)) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.messages = append(d.messages, msg)
	if d.timer != nil {
		stopSucceeded := d.timer.Stop()
		if stopSucceeded {
			wg.Done()
		}
	}

	wg.Add(1)
	d.timer = time.AfterFunc(d.interval, func() {
		defer wg.Done()
		d.mu.Lock()
		messagesCopy := make([]T, len(d.messages))
		copy(messagesCopy, d.messages)
		d.messages = nil
		d.mu.Unlock()

		f(messagesCopy)
	})
}

func subscriber(msgCh chan string) {
	wg := &sync.WaitGroup{}
	name := "sub: "

	debouncer := NewDebouncer[string](5 * time.Second)

	for msg := range msgCh {
		fmt.Println(name, "Receive msg", msg)
		debouncer.Debounce(
			msg,
			wg,
			func(messages []string) {
				fmt.Println(name, "Debounced func start", messages)
				time.Sleep(5 * time.Second)
				fmt.Println(name, "Debounced func end", messages)
			})
	}

	fmt.Println(name, "Stopping ...")
	debouncer.mu.Lock()
	if debouncer.timer != nil {
		stopSucceeded := debouncer.timer.Stop()
		if stopSucceeded {
			fmt.Println(name, "Debounced func canceled", debouncer.messages)
			wg.Done()
		}
	}
	debouncer.mu.Unlock()
	wg.Wait()
	fmt.Println(name, "Stopped")
}

func publisher(msgCh chan string, stopCh chan chan struct{}) {
	name := "pub: "
	inputCh := make(chan string)
	go func() {
		var input string
		for {
			fmt.Scanln(&input)
			inputCh <- input
		}
	}()

	for {
		select {
		case input := <-inputCh:
			msgCh <- input
		case doneCh := <-stopCh:
			fmt.Println(name, "Receive stop")
			doneCh <- struct{}{}
			return
		}
	}
}

func main() {
	fmt.Println("--- Start ---")
	defer fmt.Println("--- End ---")

	stopPubCh := make(chan chan struct{})

	msgCh := make(chan string)
	subWg := &sync.WaitGroup{}

	subWg.Add(1)
	go func() {
		defer subWg.Done()
		subscriber(msgCh)
	}()
	go publisher(msgCh, stopPubCh)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)
	s := <-sig
	fmt.Printf("Signal received: %s \n", s.String())

	doneCh := make(chan struct{})
	stopPubCh <- doneCh
	<-doneCh
	close(stopPubCh)
	close(doneCh)
	fmt.Println("Stopped publisher")

	close(msgCh)
	subWg.Wait()
	fmt.Println("Stopped subscriber")

}
