//go:build integration

package web

import "golang.org/x/time/rate"

// newUnlimitedLimiter returns a rate.Limiter that never blocks.
// Used in integration tests to prevent rate-limiting from gating rapid
// sequential calls within a single test.
func newUnlimitedLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Inf, 1)
}
