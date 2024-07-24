package aimd

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAdditiveIncreaseMultiplicativeDecrease(t *testing.T) {
	limiter := NewLimiter(16, 1)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()

	require.Equal(t, 1, limiter.Limit())
	require.Equal(t, 0, limiter.Acquired())
	require.NoError(t, limiter.Acquire(ctx, 1))

	require.Equal(t, 1, limiter.Limit())
	require.Equal(t, 1, limiter.Acquired())
	require.False(t, limiter.TryAcquire(1))

	limiter.ReleaseSuccess(1)
	require.Equal(t, 2, limiter.Limit())
	require.Equal(t, 0, limiter.Acquired())
	require.NoError(t, limiter.Acquire(ctx, 2))
	require.False(t, limiter.TryAcquire(1))
	require.Equal(t, 2, limiter.Limit())
	require.Equal(t, 2, limiter.Acquired())

	limiter.ReleaseSuccess(1)
	require.Equal(t, 3, limiter.Limit())
	require.Equal(t, 1, limiter.Acquired())
	require.NoError(t, limiter.Acquire(ctx, 2))
	require.False(t, limiter.TryAcquire(1))
	require.Equal(t, 3, limiter.Limit())
	require.Equal(t, 3, limiter.Acquired())

	limiter.ReleaseFailure(1)
	require.Equal(t, 1, limiter.Limit())
	require.Equal(t, 2, limiter.Acquired())
	require.False(t, limiter.TryAcquire(1))

	limiter.ReleaseFailure(2)
	require.Equal(t, 1, limiter.Limit())
	require.Equal(t, 0, limiter.Acquired())
	require.NoError(t, limiter.Acquire(ctx, 1))
	require.False(t, limiter.TryAcquire(1))
	require.Equal(t, 1, limiter.Limit())
	require.Equal(t, 1, limiter.Acquired())
}

func TestAdditiveIncreaseMultiplicativeDecrease_Concurrent(t *testing.T) {
	workers := 10
	limiter := NewLimiter(workers, 1)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()

	// Skip the slow start phase.
	for limiter.Limit() < workers {
		require.NoError(t, limiter.Acquire(ctx, 1))
		limiter.ReleaseSuccess(1)
	}

	for j := 0; j < workers; j++ {
		require.NoError(t, limiter.Acquire(ctx, 1))

		go func() {
			limiter.Release(1)
		}()
	}

	require.NoError(t, limiter.Acquire(ctx, int64(workers)))
	require.Equal(t, workers, limiter.Acquired())
}
