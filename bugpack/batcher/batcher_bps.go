//go:build ignore

package main

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"time"

	"github.com/shagohead/bugpack/bugpack/batcher"
	"github.com/shagohead/bugpack/bugpack/envelope"
)

func main() {
	var (
		capacity int
		parallel int
		details  bool
	)
	flag.IntVar(&capacity, "c", 1000, "Buffers capacity")
	flag.IntVar(&parallel, "p", 1, "Parallel senders")
	flag.BoolVar(&details, "d", false, "Print detailed counters from senders")
	flag.Parse()

	batcher := batcher.New(func() batcher.Buffer {
		b := make(arrbuffer, 0, capacity)
		return &b
	}, batcher.DefaultConfig)
	go func() {
		batcher.Serve()
	}()
	event := new(envelope.Envelope)
	counters := make(chan int, parallel)
	var wg sync.WaitGroup
	start := time.Now()
	for range parallel {
		wg.Go(func() {
			var c int
			s := time.Now().Add(time.Second)
			for {
				for range 1000 {
					batcher.Batch(event)
					c++
				}
				if !time.Now().Before(s) {
					counters <- c
					return
				}
			}
		})
	}
	wg.Wait()
	duration := time.Now().Sub(start)
	close(counters)
	var total int
	for c := range counters {
		if details {
			fmt.Printf("Calls by one sender: %v\n", c)
		}
		total += c
	}
	fmt.Printf("Measured Batch calls: %v/%v\n", total, duration)
}

type arrbuffer []*envelope.Envelope

// Append implements Buffer.
func (a *arrbuffer) Append(e *envelope.Envelope) bool {
	*a = append(*a, e)
	return len(*a) == cap(*a)
}

// Empty implements Buffer.
func (a *arrbuffer) Empty() bool {
	return len(*a) == 0
}

// Flush implements Buffer.
func (a *arrbuffer) Flush(context.Context) error {
	*a = (*a)[:0]
	return nil
}
