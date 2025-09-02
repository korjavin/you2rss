package middleware

import (
	"net/http"
	"sync"

	"golang.org/x/time/rate"
	"yt-podcaster/internal/models"
)

// RateLimiterMiddleware holds the rate limiters for each user.
type RateLimiterMiddleware struct {
	limiters map[int64]*rate.Limiter
	mu       sync.Mutex
	// Rate is the number of events per second.
	rate rate.Limit
	// Burst is the burst size.
	burst int
}

// NewRateLimiterMiddleware creates a new RateLimiterMiddleware.
func NewRateLimiterMiddleware(r rate.Limit, b int) *RateLimiterMiddleware {
	return &RateLimiterMiddleware{
		limiters: make(map[int64]*rate.Limiter),
		rate:     r,
		burst:    b,
	}
}

// Middleware is the actual middleware handler.
func (rl *RateLimiterMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(UserContextKey).(*models.User)
		if !ok {
			// This should not happen if AuthMiddleware is used before this.
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		rl.mu.Lock()
		limiter, exists := rl.limiters[user.ID]
		if !exists {
			limiter = rate.NewLimiter(rl.rate, rl.burst)
			rl.limiters[user.ID] = limiter
		}
		rl.mu.Unlock()

		if !limiter.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
