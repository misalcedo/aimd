package aimd

import (
	"container/list"
	"context"
	"sync"
)

type waiter struct {
	spots int
	ready chan<- struct{}
}

type AdditiveIncreaseMultiplicativeDecrease struct {
	size     int
	increase int
	decrease int
	acquired int
	mutex    sync.Mutex
	waiters  list.List
}

func NewAdditiveIncreaseMultiplicativeDecrease(size int, increase int, decrease int) *AdditiveIncreaseMultiplicativeDecrease {
	return &AdditiveIncreaseMultiplicativeDecrease{
		size:     size,
		increase: increase,
		decrease: decrease,
	}
}

func (a *AdditiveIncreaseMultiplicativeDecrease) Size() int {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.size
}

func (a *AdditiveIncreaseMultiplicativeDecrease) Acquired() int {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.acquired
}

func (a *AdditiveIncreaseMultiplicativeDecrease) Acquire(ctx context.Context, spots int) error {
	done := ctx.Done()

	a.mutex.Lock()
	select {
	case <-done:
		a.mutex.Unlock()
		return ctx.Err()
	default:
	}

	if a.acquired+spots <= a.size && a.waiters.Len() == 0 {
		a.acquired += spots
		a.mutex.Unlock()
		return nil
	}

	if spots > a.size {
		a.mutex.Unlock()
		<-done
		return ctx.Err()
	}

	ready := make(chan struct{})
	w := waiter{spots: spots, ready: ready}
	element := a.waiters.PushBack(w)
	a.mutex.Unlock()

	select {
	case <-done:
		a.mutex.Lock()

		select {
		case <-ready:
			a.acquired -= spots
			a.notifyWaiters()
		default:
			isFront := a.waiters.Front() == element
			a.waiters.Remove(element)
			if isFront && a.size > a.acquired {
				a.notifyWaiters()
			}
		}
		a.mutex.Unlock()
		return ctx.Err()

	case <-ready:
		select {
		case <-done:
			a.Release(spots)
			return ctx.Err()
		default:
		}
		return nil
	}
}

func (a *AdditiveIncreaseMultiplicativeDecrease) TryAcquire(spots int) bool {
	a.mutex.Lock()
	success := a.acquired+spots <= a.size && a.waiters.Len() == 0
	if success {
		a.acquired += spots
	}
	a.mutex.Unlock()
	return success
}

func (a *AdditiveIncreaseMultiplicativeDecrease) Release(spots int) {
	a.mutex.Lock()
	if a.acquired < spots {
		a.mutex.Unlock()
		panic("released more than held")
	}

	a.acquired -= spots
	a.notifyWaiters()
	a.mutex.Unlock()
}

func (a *AdditiveIncreaseMultiplicativeDecrease) ReleaseSuccess(spots int) {
	a.mutex.Lock()
	if a.acquired < spots {
		a.mutex.Unlock()
		panic("released more than held")
	}

	a.acquired -= spots
	a.size += a.increase
	a.notifyWaiters()
	a.mutex.Unlock()
}

func (a *AdditiveIncreaseMultiplicativeDecrease) ReleaseFailure(spots int, minSize int) {
	a.mutex.Lock()
	if a.acquired < spots {
		a.mutex.Unlock()
		panic("released more than held")
	}

	a.acquired -= spots
	a.size = max(minSize, a.size/a.decrease)
	a.notifyWaiters()
	a.mutex.Unlock()
}

func (a *AdditiveIncreaseMultiplicativeDecrease) notifyWaiters() {
	for {
		next := a.waiters.Front()
		if next == nil {
			break
		}

		w := next.Value.(waiter)
		if (a.acquired + w.spots) >= a.size {
			break
		}

		a.acquired += w.spots
		a.waiters.Remove(next)
		close(w.ready)
	}
}
