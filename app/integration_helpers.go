//go:build integration

package app

import "golang.org/x/time/rate"

// SetRateLimiterForTesting replaces the App's rate limiter with the given one.
// Only compiled under the integration build tag; used by integration tests in
// other packages that need to override rate limiting without touching unexported
// fields directly.
func SetRateLimiterForTesting(a *App, l *rate.Limiter) {
	a.rateLimiter = l
}
