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
		opts.KeyGen = func(ctx *ExecutionContext) string {
			return ctx.Context.Get("X-Real-IP").(string)
		}
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
		Limit: limit,
		TTL:   ttl,
		KeyGen: func(ctx *ExecutionContext) string {
			// Try multiple headers for IP
			ip := ctx.Context.Get("X-Real-IP")
			if ip == "" {
				ip = ctx.Context.Get("X-Forwarded-For")
			}
			if ip == "" {
				ip = ctx.Context.Get("RemoteAddr")
			}
			return ip.(string)
		},
	})
}

// UserThrottler creates a rate limiter based on user ID
func UserThrottler(limit int, ttl time.Duration) *ThrottlerGuard {
	return NewThrottlerGuard(&ThrottlerGuardOptions{
		Limit: limit,
		TTL:   ttl,
		KeyGen: func(ctx *ExecutionContext) string {
			userID, _ := ctx.Context.Get("user:id").(string)
			return fmt.Sprintf("user:%s", userID)
		},
	})
}
