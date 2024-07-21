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
	var producers sync.WaitGroup
	var concurrency, iterations, rate, workers int
	var errorDuration, successDuration time.Duration

	flag.IntVar(&iterations, "iterations", 100, "number of iterations")
	flag.IntVar(&rate, "rate", 100, "rate of the underlying resource")
	flag.IntVar(&workers, "workers", 10, "number of workers")
	flag.IntVar(&concurrency, "concurrency", 100, "concurrency of a worker")
	flag.DurationVar(&successDuration, "successDuration", 100*time.Millisecond, "duration of success")
	flag.DurationVar(&errorDuration, "errorDuration", time.Second*1, "duration of error")
	flag.Parse()

	if rate >= workers*concurrency {
		log.Fatalln("rate must be less than the workers multiplied by their concurrency")
	}

	root := aimd.NewLimiter(rate, 1)

	start := time.Now()
	metrics := make(chan [4]int, workers*concurrency*iterations)

	for i := 0; i < workers; i++ {
		limiter := aimd.NewLimiter(1, 1)

		for j := 0; j < concurrency; j++ {
			producers.Add(1)
			go func(id int) {
				defer producers.Done()

				for k := 0; k < iterations; k++ {
					_ = limiter.Acquire(context.Background(), 1)

					metrics <- [4]int{int(time.Since(start).Nanoseconds()), id, 1, root.Acquired()}

					if root.TryAcquire(1) {
						time.Sleep(successDuration)
						limiter.ReleaseSuccess(1)
						root.Release(1)
					} else {
						time.Sleep(errorDuration)
						limiter.ReleaseFailure(1)
					}
				}
			}(i)
		}
	}

	var printer sync.WaitGroup
	printer.Add(1)
	go func() {
		defer printer.Done()
		fmt.Println("X,ID,Client,Server")
		for entry := range metrics {
			fmt.Printf("%d,%d,%d,%d\n", entry[0]/35_000, entry[1], entry[2], entry[3])
		}
	}()

	producers.Wait()
	close(metrics)
	printer.Wait()
}
