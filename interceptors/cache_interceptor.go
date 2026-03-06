// gonest/interceptors/cache_interceptor.go
package interceptors

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CacheInterceptor caches responses
type CacheInterceptor struct {
	ttl    time.Duration
	keyGen func(*ExecutionContext) string
	store  *cacheStore
}

// cacheStore stores cached responses
type cacheStore struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	response  any
	expiresAt time.Time
}

// CacheInterceptorOptions configures the cache interceptor
type CacheInterceptorOptions struct {
	TTL    time.Duration
	KeyGen func(*ExecutionContext) string
}

// NewCacheInterceptor creates a new cache interceptor
func NewCacheInterceptor(opts *CacheInterceptorOptions) *CacheInterceptor {
	if opts.TTL <= 0 {
		opts.TTL = 5 * time.Minute
	}

	if opts.KeyGen == nil {
		// Default: generate key from method + path
		opts.KeyGen = func(ctx *ExecutionContext) string {
			method := ctx.Context.Get("method")
			path := ctx.Context.Get("path")
			return fmt.Sprintf("%s:%s", method, path)
		}
	}

	interceptor := &CacheInterceptor{
		ttl:    opts.TTL,
		keyGen: opts.KeyGen,
		store: &cacheStore{
			entries: make(map[string]*cacheEntry),
		},
	}

	// Start cleanup goroutine
	go interceptor.cleanup()

	return interceptor
}

// Intercept checks cache or executes handler
func (i *CacheInterceptor) Intercept(ctx *ExecutionContext, next func() error) error {
	key := i.keyGen(ctx)

	// Check cache
	i.store.mu.RLock()
	entry, exists := i.store.entries[key]
	i.store.mu.RUnlock()

	if exists && time.Now().Before(entry.expiresAt) {
		// Cache hit - store in context
		ctx.Context.Set("cache:hit", true)
		ctx.Context.Set("cache:response", entry.response)
		return nil
	}

	// Cache miss - execute handler
	ctx.Context.Set("cache:hit", false)
	err := next()

	if err == nil {
		// Store response in cache (simplified - would need actual response capture)
		i.store.mu.Lock()
		i.store.entries[key] = &cacheEntry{
			response:  nil, // Would capture actual response
			expiresAt: time.Now().Add(i.ttl),
		}
		i.store.mu.Unlock()
	}

	return err
}

// cleanup periodically removes expired entries
func (i *CacheInterceptor) cleanup() {
	ticker := time.NewTicker(i.ttl)
	defer ticker.Stop()

	for range ticker.C {
		i.store.mu.Lock()
		now := time.Now()

		for key, entry := range i.store.entries {
			if now.After(entry.expiresAt) {
				delete(i.store.entries, key)
			}
		}

		i.store.mu.Unlock()
	}
}

// SimpleCacheInterceptor creates a basic cache interceptor
func SimpleCacheInterceptor(ttl time.Duration) *CacheInterceptor {
	return NewCacheInterceptor(&CacheInterceptorOptions{
		TTL: ttl,
	})
}

// CacheKeyFromBody generates cache key including request body
func CacheKeyFromBody() func(*ExecutionContext) string {
	return func(ctx *ExecutionContext) string {
		method := ctx.Context.Get("method")
		path := ctx.Context.Get("path")

		// Get body from context if available
		body := ctx.Context.Get("body")
		if body != nil {
			bodyJSON, _ := json.Marshal(body)
			hash := md5.Sum(bodyJSON)
			return fmt.Sprintf("%s:%s:%x", method, path, hash)
		}

		return fmt.Sprintf("%s:%s", method, path)
	}
}
