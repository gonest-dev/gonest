// gonest/interceptors/cache_interceptor.go
package interceptors

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ============================================================
// CACHE STORAGE INTERFACE
// ============================================================

// CacheStorage interface for pluggable cache implementations
type CacheStorage interface {
	// Get retrieves a value from cache
	Get(ctx context.Context, key string) (any, bool, error)

	// Set stores a value in cache with TTL
	Set(ctx context.Context, key string, value any, ttl time.Duration) error

	// Delete removes a value from cache
	Delete(ctx context.Context, key string) error

	// Clear removes all entries from cache
	Clear(ctx context.Context) error

	// Has checks if key exists
	Has(ctx context.Context, key string) (bool, error)
}

// ============================================================
// IN-MEMORY STORAGE (Default)
// ============================================================

// InMemoryCacheStorage provides in-memory cache storage
type InMemoryCacheStorage struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	value     any
	expiresAt time.Time
}

// NewInMemoryCacheStorage creates a new in-memory cache storage
func NewInMemoryCacheStorage() *InMemoryCacheStorage {
	storage := &InMemoryCacheStorage{
		entries: make(map[string]*cacheEntry),
	}

	// Start cleanup goroutine
	go storage.cleanup()

	return storage
}

// Get retrieves a value from in-memory cache
func (s *InMemoryCacheStorage) Get(ctx context.Context, key string) (any, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[key]
	if !exists {
		return nil, false, nil
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		return nil, false, nil
	}

	return entry.value, true, nil
}

// Set stores a value in in-memory cache
func (s *InMemoryCacheStorage) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[key] = &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}

	return nil
}

// Delete removes a value from in-memory cache
func (s *InMemoryCacheStorage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, key)
	return nil
}

// Clear removes all entries from in-memory cache
func (s *InMemoryCacheStorage) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = make(map[string]*cacheEntry)
	return nil
}

// Has checks if key exists in in-memory cache
func (s *InMemoryCacheStorage) Has(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[key]
	if !exists {
		return false, nil
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		return false, nil
	}

	return true, nil
}

// cleanup periodically removes expired entries
func (s *InMemoryCacheStorage) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()

		for key, entry := range s.entries {
			if now.After(entry.expiresAt) {
				delete(s.entries, key)
			}
		}

		s.mu.Unlock()
	}
}

// ============================================================
// NO-OP STORAGE (Disabled cache)
// ============================================================

// NoOpCacheStorage is a cache storage that does nothing (disables caching)
type NoOpCacheStorage struct{}

// NewNoOpCacheStorage creates a no-op cache storage
func NewNoOpCacheStorage() *NoOpCacheStorage {
	return &NoOpCacheStorage{}
}

func (s *NoOpCacheStorage) Get(ctx context.Context, key string) (any, bool, error) {
	return nil, false, nil
}

func (s *NoOpCacheStorage) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return nil
}

func (s *NoOpCacheStorage) Delete(ctx context.Context, key string) error {
	return nil
}

func (s *NoOpCacheStorage) Clear(ctx context.Context) error {
	return nil
}

func (s *NoOpCacheStorage) Has(ctx context.Context, key string) (bool, error) {
	return false, nil
}

// ============================================================
// REDIS STORAGE (Example - commented out)
// ============================================================

// RedisCacheStorage provides Redis-based cache storage
// To use, uncomment and import: github.com/redis/go-redis/v9
//
// Example implementation:
/*
import (
	"github.com/redis/go-redis/v9"
)

type RedisCacheStorage struct {
	client redis.UniversalClient
	prefix string
}

func NewRedisCacheStorage(client redis.UniversalClient, prefix string) *RedisCacheStorage {
	if prefix == "" {
		prefix = "cache:"
	}
	return &RedisCacheStorage{
		client: client,
		prefix: prefix,
	}
}

func (s *RedisCacheStorage) Get(ctx context.Context, key string) (any, bool, error) {
	fullKey := s.prefix + key
	val, err := s.client.Get(ctx, fullKey).Result()

	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var result any
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return nil, false, err
	}

	return result, true, nil
}

func (s *RedisCacheStorage) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	fullKey := s.prefix + key

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, fullKey, data, ttl).Err()
}

func (s *RedisCacheStorage) Delete(ctx context.Context, key string) error {
	fullKey := s.prefix + key
	return s.client.Del(ctx, fullKey).Err()
}

func (s *RedisCacheStorage) Clear(ctx context.Context) error {
	pattern := s.prefix + "*"
	iter := s.client.Scan(ctx, 0, pattern, 0).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return err
	}

	if len(keys) > 0 {
		return s.client.Del(ctx, keys...).Err()
	}

	return nil
}

func (s *RedisCacheStorage) Has(ctx context.Context, key string) (bool, error) {
	fullKey := s.prefix + key
	exists, err := s.client.Exists(ctx, fullKey).Result()
	return exists > 0, err
}
*/

// ============================================================
// CACHE INTERCEPTOR
// ============================================================

// CacheInterceptor caches responses using pluggable storage
type CacheInterceptor struct {
	ttl     time.Duration
	keyGen  func(*ExecutionContext) string
	storage CacheStorage
}

// CacheInterceptorOptions configures the cache interceptor
type CacheInterceptorOptions struct {
	TTL     time.Duration
	KeyGen  func(*ExecutionContext) string
	Storage CacheStorage // Pluggable storage backend
}

// NewCacheInterceptor creates a new cache interceptor
func NewCacheInterceptor(opts *CacheInterceptorOptions) *CacheInterceptor {
	if opts.TTL <= 0 {
		opts.TTL = 5 * time.Minute
	}

	if opts.KeyGen == nil {
		// Default: generate key from method + path
		opts.KeyGen = func(ctx *ExecutionContext) string {
			method := ctx.Context.Method()
			path := ctx.Context.Path()
			return fmt.Sprintf("%s:%s", method, path)
		}
	}

	if opts.Storage == nil {
		// Default: in-memory storage
		opts.Storage = NewInMemoryCacheStorage()
	}

	return &CacheInterceptor{
		ttl:     opts.TTL,
		keyGen:  opts.KeyGen,
		storage: opts.Storage,
	}
}

// Intercept checks cache or executes handler
func (i *CacheInterceptor) Intercept(ctx *ExecutionContext, next func() error) error {
	cacheCtx := context.Background()
	key := i.keyGen(ctx)

	// Check cache
	cachedValue, exists, err := i.storage.Get(cacheCtx, key)
	if err != nil {
		// Log error but continue (cache failure shouldn't break request)
		ctx.Context.Set("cache:error", err.Error())
	}

	if exists && err == nil {
		// Cache hit - store in context
		ctx.Context.Set("cache:hit", true)
		ctx.Context.Set("cache:response", cachedValue)

		// Return cached response
		if cachedValue != nil {
			return ctx.Context.JSON(200, cachedValue)
		}
		return nil
	}

	// Cache miss - execute handler
	ctx.Context.Set("cache:hit", false)
	err = next()

	if err == nil {
		// Store response in cache
		// Note: This is simplified - real implementation would capture actual response
		response := ctx.Context.Get("response")
		if response != nil {
			if setErr := i.storage.Set(cacheCtx, key, response, i.ttl); setErr != nil {
				ctx.Context.Set("cache:error", setErr.Error())
			}
		}
	}

	return err
}

// ============================================================
// HELPER FUNCTIONS
// ============================================================

// SimpleCacheInterceptor creates a basic in-memory cache interceptor
func SimpleCacheInterceptor(ttl time.Duration) *CacheInterceptor {
	return NewCacheInterceptor(&CacheInterceptorOptions{
		TTL: ttl,
	})
}

// CacheKeyFromBody generates cache key including request body
func CacheKeyFromBody() func(*ExecutionContext) string {
	return func(ctx *ExecutionContext) string {
		method := ctx.Context.Method()
		path := ctx.Context.Path()

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

// CacheKeyFromQuery generates cache key including query parameters
// Note: This creates a hash of the full query string
func CacheKeyFromQuery() func(*ExecutionContext) string {
	return func(ctx *ExecutionContext) string {
		method := ctx.Context.Method()
		path := ctx.Context.Path()

		// Get raw query string from context
		// Different platforms store this differently, so we try multiple approaches
		queryString := ""

		// Try to get from context first
		if qs := ctx.Context.GetString("query"); qs != "" {
			queryString = qs
		}

		if queryString != "" {
			hash := md5.Sum([]byte(queryString))
			return fmt.Sprintf("%s:%s:q:%x", method, path, hash)
		}

		return fmt.Sprintf("%s:%s", method, path)
	}
}

// CacheKeyFromQueryParams generates cache key from specific query parameters
// Usage: CacheKeyFromQueryParams("page", "limit", "sort")
func CacheKeyFromQueryParams(params ...string) func(*ExecutionContext) string {
	return func(ctx *ExecutionContext) string {
		method := ctx.Context.Method()
		path := ctx.Context.Path()

		if len(params) == 0 {
			return fmt.Sprintf("%s:%s", method, path)
		}

		// Build query key from specified params
		var queryParts []string
		for _, param := range params {
			value := ctx.Context.Query(param)
			if value != "" {
				queryParts = append(queryParts, fmt.Sprintf("%s=%s", param, value))
			}
		}

		if len(queryParts) > 0 {
			queryStr := fmt.Sprintf("%v", queryParts)
			hash := md5.Sum([]byte(queryStr))
			return fmt.Sprintf("%s:%s:q:%x", method, path, hash)
		}

		return fmt.Sprintf("%s:%s", method, path)
	}
}

// CacheKeyFromUser generates cache key per user
func CacheKeyFromUser() func(*ExecutionContext) string {
	return func(ctx *ExecutionContext) string {
		method := ctx.Context.Method()
		path := ctx.Context.Path()

		// Get user ID from context
		userID := ctx.Context.GetString("user:id")
		if userID != "" {
			return fmt.Sprintf("%s:%s:user:%s", method, path, userID)
		}

		return fmt.Sprintf("%s:%s", method, path)
	}
}


