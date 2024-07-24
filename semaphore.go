// Package aimd provides a weighted semaphore implementation with support for Additive increase/multiplicative decrease.
package aimd

import (
	"math"
)

// Limiter provides a way to bound concurrent access to a resource.
// The callers can request access with a given weight.
type Limiter struct {
	Weighted
	congestionWindow   float64
	slowStartThreshold float64
	maximumWeight      float64
}

// NewLimiter creates a new weighted semaphore with the given
// maximum combined weight for concurrent access.
// Unlike [golang.org/x/sync/semaphore], this semaphore supports a dynamic maximum weight.
// On [Limiter.ReleaseSuccess], the maximum combined weight is increased linearly.
// On [Limiter.ReleaseFailure], the maximum combined weight is decreased exponentially.
func NewLimiter(slowStartThreshold int, maximumWeight int) *Limiter {
	return &Limiter{
		Weighted:           Weighted{size: 1},
		congestionWindow:   1.0,
		slowStartThreshold: float64(slowStartThreshold),
		maximumWeight:      float64(maximumWeight),
	}
}

// Limit is the maximum combined weight for concurrent access allowed by this semaphore.
func (a *Limiter) Limit() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return int(a.size)
}

// Acquired is the current combined weight for concurrent access in use by this semaphore.
// The semaphore may be over-subscribed ([Limiter.Limit] > [Limiter.Acquired]) when the limit is reduced.
func (a *Limiter) Acquired() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return int(a.cur)
}

// ReleaseFailure releases the semaphore with a weight of n and decreases the maximum combined weight.
// The maximum combined weight is guaranteed to be greater than 0.
func (a *Limiter) ReleaseFailure(n int) {
	a.mu.Lock()
	a.cur -= int64(n)
	if a.cur < 0 {
		a.mu.Unlock()
		panic("semaphore: released more than held")
	}

	a.slowStartThreshold = max(a.congestionWindow/2, 1)
	a.congestionWindow = a.slowStartThreshold
	a.size = int64(math.Floor(a.congestionWindow))

	a.notifyWaiters()
	a.mu.Unlock()
}

// ReleaseSuccess releases the semaphore with a weight of n and increases the maximum combined weight.
func (a *Limiter) ReleaseSuccess(n int) {
	a.mu.Lock()
	a.cur -= int64(n)
	if a.cur < 0 {
		a.mu.Unlock()
		panic("semaphore: released more than held")
	}

	if a.congestionWindow > a.slowStartThreshold {
		a.congestionWindow += (a.maximumWeight * a.maximumWeight / a.congestionWindow) + (a.maximumWeight / 8)
	} else {
		a.congestionWindow += a.maximumWeight
	}

	a.size = int64(math.Floor(a.congestionWindow))

	a.notifyWaiters()
	a.mu.Unlock()
}
