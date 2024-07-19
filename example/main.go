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

	flag.IntVar(&iterations, "iterations", 100, "number of iterations")
	flag.IntVar(&rate, "rate", 100, "rate of the underlying resource")
	flag.IntVar(&workers, "workers", 10, "number of workers")
	flag.IntVar(&concurrency, "concurrency", 100, "concurrency of a worker")
	flag.Parse()

	if rate >= workers*concurrency {
		log.Fatalln("rate must be less than the workers multiplied by their concurrency")
	}

	root := aimd.NewLimiter(rate, 0, 1)

	start := time.Now()
	metrics := make(chan [4]int, workers*concurrency*iterations)

	for i := 0; i < workers; i++ {
		limiter := aimd.NewLimiter(1, 1, 2)

		for j := 0; j < concurrency; j++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for k := 0; k < iterations; k++ {
					_ = limiter.Acquire(context.Background(), 1)

					metrics <- [4]int{int(time.Since(start).Nanoseconds()), id, 1, root.Acquired()}

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

	close(metrics)

	fmt.Println("X,ID,Client,Server")
	for entry := range metrics {
		fmt.Printf("%d,%d,%d,%d\n", entry[0]%10_000, entry[1], entry[2], entry[3])
	}
}
