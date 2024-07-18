package aimd

import (
	"time"
)

type TokenBucket struct {
	lastFill  time.Time
	rate      time.Duration
	maxTokens uint
	tokens    uint
	fraction  time.Duration
}

func NewTokenBucket(rate time.Duration, maxTokens uint) *TokenBucket {
	return &TokenBucket{
		rate:      rate,
		maxTokens: maxTokens,
		tokens:    0,
		fraction:  0,
	}
}

func (b *TokenBucket) Drain(now time.Time) {
	b.lastFill = now
	b.tokens = 0
	b.fraction = 0
}

func (b *TokenBucket) Debit(now time.Time, tokens uint) bool {
	b.fill(now)

	sufficientFunds := b.tokens >= tokens

	if sufficientFunds {
		b.tokens -= tokens
	}

	return sufficientFunds
}

func (b *TokenBucket) fill(now time.Time) {
	elapsed := now.Sub(b.lastFill) + b.fraction

	b.fraction = 0
	b.lastFill = now

	b.tokens = min(b.maxTokens, b.tokens+uint(elapsed/b.rate))
	b.fraction = elapsed % b.rate
}
