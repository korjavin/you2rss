package middleware

import (
	"log"
	"net/http"
	"sync"

	"yt-podcaster/internal/models"

	"golang.org/x/time/rate"
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
		log.Printf("RateLimiter: Processing %s %s", r.Method, r.URL.Path)
		user, ok := r.Context().Value(models.UserContextKey).(*models.User)
		if !ok {
			log.Printf("RateLimiter: No user in context - unauthorized")
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
			log.Printf("RateLimiter: Rate limit exceeded for user %d", user.ID)
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		log.Printf("RateLimiter: Calling next handler for user %d", user.ID)
		next.ServeHTTP(w, r)
		log.Printf("RateLimiter: Completed request for user %d", user.ID)
	})
}
