// gonest/guards/throttler_guard.go
package guards

import (
	"fmt"
	"sync"
	"time"
)

// ThrottlerGuard implements rate limiting
type ThrottlerGuard struct {
	limit  int
	ttl    time.Duration
	keyGen func(*ExecutionContext) string
	store  *throttlerStore
}

// throttlerStore stores request counts
type throttlerStore struct {
	mu      sync.RWMutex
	entries map[string]*throttlerEntry
}

type throttlerEntry struct {
	count     int
	expiresAt time.Time
}

// ThrottlerGuardOptions configures the throttler guard
type ThrottlerGuardOptions struct {
	Limit  int           // Max requests
	TTL    time.Duration // Time window
	KeyGen func(*ExecutionContext) string
}

// NewThrottlerGuard creates a new rate limiting guard
func NewThrottlerGuard(opts *ThrottlerGuardOptions) *ThrottlerGuard {
	if opts.Limit <= 0 {
		opts.Limit = 10
	}

	if opts.TTL <= 0 {
		opts.TTL = time.Minute
	}

	if opts.KeyGen == nil {
		// Default: use IP address as key
		opts.KeyGen = getClientIP
	}

	guard := &ThrottlerGuard{
		limit:  opts.Limit,
		ttl:    opts.TTL,
		keyGen: opts.KeyGen,
		store: &throttlerStore{
			entries: make(map[string]*throttlerEntry),
		},
	}

	// Start cleanup goroutine
	go guard.cleanup()

	return guard
}

// CanActivate checks if request is within rate limit
func (g *ThrottlerGuard) CanActivate(ctx *ExecutionContext) (bool, error) {
	key := g.keyGen(ctx)
	if key == "" {
		// No key, allow request
		return true, nil
	}

	g.store.mu.Lock()
	defer g.store.mu.Unlock()

	now := time.Now()
	entry, exists := g.store.entries[key]

	if !exists || now.After(entry.expiresAt) {
		// First request or expired entry
		g.store.entries[key] = &throttlerEntry{
			count:     1,
			expiresAt: now.Add(g.ttl),
		}
		return true, nil
	}

	// Check if limit exceeded
	if entry.count >= g.limit {
		retryAfter := int(entry.expiresAt.Sub(now).Seconds())

		return false, NewGuardError("Too many requests", 429).
			WithDetail("limit", g.limit).
			WithDetail("retryAfter", retryAfter)
	}

	// Increment counter
	entry.count++

	return true, nil
}

// cleanup periodically removes expired entries
func (g *ThrottlerGuard) cleanup() {
	ticker := time.NewTicker(g.ttl)
	defer ticker.Stop()

	for range ticker.C {
		g.store.mu.Lock()
		now := time.Now()

		for key, entry := range g.store.entries {
			if now.After(entry.expiresAt) {
				delete(g.store.entries, key)
			}
		}

		g.store.mu.Unlock()
	}
}

// getClientIP extracts client IP from various headers
func getClientIP(ctx *ExecutionContext) string {
	// Try X-Real-IP header first
	if ip := ctx.Context.Header("X-Real-IP"); ip != "" {
		return ip
	}

	// Try X-Forwarded-For header
	if ip := ctx.Context.Header("X-Forwarded-For"); ip != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		return ip
	}

	// Fallback to request's remote address
	// Note: This might need adjustment based on your core.Context implementation
	if ip := ctx.Context.GetString("remote_addr"); ip != "" {
		return ip
	}

	return ""
}

// SimpleThrottler creates a basic rate limiter
func SimpleThrottler(limit int, ttl time.Duration) *ThrottlerGuard {
	return NewThrottlerGuard(&ThrottlerGuardOptions{
		Limit: limit,
		TTL:   ttl,
	})
}

// IPThrottler creates a rate limiter based on IP address
func IPThrottler(limit int, ttl time.Duration) *ThrottlerGuard {
	return NewThrottlerGuard(&ThrottlerGuardOptions{
		Limit:  limit,
		TTL:    ttl,
		KeyGen: getClientIP,
	})
}

// UserThrottler creates a rate limiter based on user ID
func UserThrottler(limit int, ttl time.Duration) *ThrottlerGuard {
	return NewThrottlerGuard(&ThrottlerGuardOptions{
		Limit: limit,
		TTL:   ttl,
		KeyGen: func(ctx *ExecutionContext) string {
			userID := ctx.Context.GetString("user:id")
			if userID == "" {
				return ""
			}
			return fmt.Sprintf("user:%s", userID)
		},
	})
}
