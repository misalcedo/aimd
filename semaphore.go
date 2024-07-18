// Package aimd provides a weighted semaphore implementation with support for Additive increase/multiplicative decrease.
package aimd

import (
	"container/list"
	"context"
	"sync"
)

// Limiter provides a way to bound concurrent access to a resource.
// The callers can request access with a given weight.
type Limiter struct {
	size     int
	acquired int
	increase int
	decrease int
	mutex    sync.Mutex
	waiters  list.List
}

type waiter struct {
	n     int
	ready chan<- struct{} // Closed when semaphore acquired.
}

// NewLimiter creates a new weighted semaphore with the given
// maximum combined weight for concurrent access.
// Unlike [golang.org/x/sync/semaphore], this semaphore supports a dynamic maximum weight.
// On [Limiter.ReleaseSuccess], the maximum combined weight is increased by adding increase.
// On [Limiter.ReleaseFailure], the maximum combined weight is decreased by dividing by decrease and dropping the remainder.
func NewLimiter(n int, increase int, decrease int) *Limiter {
	return &Limiter{size: n, increase: increase, decrease: decrease}
}

// Size is the maximum combined weight for concurrent access allowed by this semaphore.
func (a *Limiter) Size() int {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.size
}

// Acquired is the current combined weight for concurrent access in use by this semaphore.
// The semaphore may be over-subscribed ([Limiter.Size] > [Limiter.Acquired]) when the limit is reduced.
func (a *Limiter) Acquired() int {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.acquired
}

// ReleaseSuccess releases the semaphore with a weight of n and increases the maximum combined weight..
func (a *Limiter) ReleaseSuccess(n int) {
	a.mutex.Lock()
	if a.acquired < n {
		a.mutex.Unlock()
		panic("released more than held")
	}

	a.acquired -= n
	a.size += a.increase
	a.notifyWaiters()
	a.mutex.Unlock()
}

// ReleaseFailure releases the semaphore with a weight of n and decreases the maximum combined weight.
// The maximum combined weight is guaranteed to be greater than 0.
func (a *Limiter) ReleaseFailure(n int) {
	a.mutex.Lock()
	if a.acquired < n {
		a.mutex.Unlock()
		panic("semaphore: released more than held")
	}

	a.acquired -= n
	a.size = max(1, a.size/a.decrease)
	a.notifyWaiters()
	a.mutex.Unlock()
}

// Acquire acquires the semaphore with a weight of n, blocking until resources
// are available or ctx is done. On success, returns nil. On failure, returns
// ctx.Err() and leaves the semaphore unchanged.
func (a *Limiter) Acquire(ctx context.Context, n int) error {
	done := ctx.Done()

	a.mutex.Lock()
	select {
	case <-done:
		// ctx becoming done has "happened before" acquiring the semaphore,
		// whether it became done before the call began or while we were
		// waiting for the mutex. We prefer to fail even if we could acquire
		// the mutex without blocking.
		a.mutex.Unlock()
		return ctx.Err()
	default:
	}
	if a.size-a.acquired >= n && a.waiters.Len() == 0 {
		// Since we hold a.mutex and haven't synchronized since checking done, if
		// ctx becomes done before we return here, it becoming done must have
		// "happened concurrently" with this call - it cannot "happen before"
		// we return in this branch. So, we're ok to always acquire here.
		a.acquired += n
		a.mutex.Unlock()
		return nil
	}

	if n > a.size {
		// Don't make other Acquire calls block on one that's doomed to fail.
		a.mutex.Unlock()
		<-done
		return ctx.Err()
	}

	ready := make(chan struct{})
	w := waiter{n: n, ready: ready}
	elem := a.waiters.PushBack(w)
	a.mutex.Unlock()

	select {
	case <-done:
		a.mutex.Lock()
		select {
		case <-ready:
			// Acquired the semaphore after we were canceled.
			// Pretend we didn't and put the tokens back.
			a.acquired -= n
			a.notifyWaiters()
		default:
			isFront := a.waiters.Front() == elem
			a.waiters.Remove(elem)
			// If we're at the front and there're extra tokens left, notify other waiters.
			if isFront && a.size > a.acquired {
				a.notifyWaiters()
			}
		}
		a.mutex.Unlock()
		return ctx.Err()

	case <-ready:
		// Acquired the semaphore. Check that ctx isn't already done.
		// We check the done channel instead of calling ctx.Err because we
		// already have the channel, and ctx.Err is O(n) with the nesting
		// depth of ctx.
		select {
		case <-done:
			a.Release(n)
			return ctx.Err()
		default:
		}
		return nil
	}
}

// TryAcquire acquires the semaphore with a weight of n without blocking.
// On success, returns true. On failure, returns false and leaves the semaphore unchanged.
func (a *Limiter) TryAcquire(n int) bool {
	a.mutex.Lock()
	success := a.size-a.acquired >= n && a.waiters.Len() == 0
	if success {
		a.acquired += n
	}
	a.mutex.Unlock()
	return success
}

// Release releases the semaphore with a weight of n.
func (a *Limiter) Release(n int) {
	a.mutex.Lock()
	a.acquired -= n
	if a.acquired < 0 {
		a.mutex.Unlock()
		panic("semaphore: released more than held")
	}
	a.notifyWaiters()
	a.mutex.Unlock()
}

func (a *Limiter) notifyWaiters() {
	for {
		next := a.waiters.Front()
		if next == nil {
			break // No more waiters blocked.
		}

		w := next.Value.(waiter)
		if a.size-a.acquired < w.n {
			// Not enough tokens for the next waiter.  We could keep going (to try to
			// find a waiter with a smaller request), but under load that could cause
			// starvation for large requests; instead, we leave all remaining waiters
			// blocked.
			//
			// Consider a semaphore used as a read-write lock, with N tokens, N
			// readers, and one writer.  Each reader can Acquire(1) to obtain a read
			// lock.  The writer can Acquire(N) to obtain a write lock, excluding all
			// of the readers.  If we allow the readers to jump ahead in the queue,
			// the writer will starve — there is always one token available for every
			// reader.
			break
		}

		a.acquired += w.n
		a.waiters.Remove(next)
		close(w.ready)
	}
}
