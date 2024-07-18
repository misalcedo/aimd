package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/misalcedo/aimd"
	"log"
	"sync"
	"time"
)

func main() {
	var wg sync.WaitGroup
	var concurrency, iterations, rate, workers int

	flag.IntVar(&iterations, "iterations", 1_000_000, "number of iterations")
	flag.IntVar(&rate, "rate", 100, "rate of the underlying resource")
	flag.IntVar(&workers, "workers", 10, "number of workers")
	flag.IntVar(&concurrency, "concurrency", 1_000, "concurrency of a worker")
	flag.Parse()

	if rate >= workers*concurrency {
		log.Fatalln("rate must be less than the workers multiplied by their concurrency")
	}

	root := aimd.NewLimiter(rate, 0, 1)

	for i := 0; i < workers; i++ {
		limiter := aimd.NewLimiter(1, 1, 2)

		go func(id int) {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			for range ticker.C {
				fmt.Printf("%d,%d\n", id, limiter.Size())
			}
		}(i)

		for j := 0; j < concurrency; j++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for k := 0; k < iterations; k++ {
					_ = limiter.Acquire(context.Background(), 1)

					if root.TryAcquire(1) {
						limiter.ReleaseSuccess(1)
						root.Release(1)
					} else {
						limiter.ReleaseFailure(1)
					}
				}
			}(i)
		}
	}

	wg.Wait()
}
