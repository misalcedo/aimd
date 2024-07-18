package aimd

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestNewTokenBucket(t *testing.T) {
	rate := time.Second / 100
	maxTokens := uint(5)
	refill := time.Duration(maxTokens) * rate
	bucket := NewTokenBucket(rate, maxTokens)

	now := time.Now()

	require.Greater(t, time.Since(bucket.lastFill), refill)
	require.True(t, bucket.Debit(now, maxTokens))
	require.False(t, bucket.Debit(now, 1))

	now = now.Add(refill)
	require.True(t, bucket.Debit(now, 5))

	now = now.Add(refill)
	bucket.Drain(now)
	require.False(t, bucket.Debit(now, 1))
}
