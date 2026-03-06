package core

import (
	"reflect"
	"sync"
)

// MetadataKey is a type for metadata keys
type MetadataKey string

const (
	// MetadataKeyCONTROLLER marks a type as a controller
	MetadataKeyCONTROLLER MetadataKey = "controller"

	// MetadataKeyROUTE stores route information
	MetadataKeyROUTE MetadataKey = "route"

	// MetadataKeyGUARD stores guard information
	MetadataKeyGUARD MetadataKey = "guard"

	// MetadataKeyINTERCEPTOR stores interceptor information
	MetadataKeyINTERCEPTOR MetadataKey = "interceptor"

	// MetadataKeyPIPE stores pipe information
	MetadataKeyPIPE MetadataKey = "pipe"

	// MetadataKeyPARAM stores parameter information
	MetadataKeyPARAM MetadataKey = "param"

	// MetadataKeySWAGGER stores swagger documentation
	MetadataKeySWAGGER MetadataKey = "swagger"
)

// MetadataStorage stores metadata for types
type MetadataStorage struct {
	data map[reflect.Type]map[MetadataKey]any
	mu   sync.RWMutex
}

// NewMetadataStorage creates a new metadata storage
func NewMetadataStorage() *MetadataStorage {
	return &MetadataStorage{
		data: make(map[reflect.Type]map[MetadataKey]any),
	}
}

// Set stores metadata for a type
func (ms *MetadataStorage) Set(target any, key MetadataKey, value any) {
	t := reflect.TypeOf(target)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, exists := ms.data[t]; !exists {
		ms.data[t] = make(map[MetadataKey]any)
	}

	ms.data[t][key] = value
}

// Get retrieves metadata for a type
func (ms *MetadataStorage) Get(target any, key MetadataKey) (any, bool) {
	t := reflect.TypeOf(target)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if typeData, exists := ms.data[t]; exists {
		value, exists := typeData[key]
		return value, exists
	}

	return nil, false
}

// GetAll retrieves all metadata for a type
func (ms *MetadataStorage) GetAll(target any) map[MetadataKey]any {
	t := reflect.TypeOf(target)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if typeData, exists := ms.data[t]; exists {
		// Return a copy to prevent external modification
		result := make(map[MetadataKey]any)
		for k, v := range typeData {
			result[k] = v
		}
		return result
	}

	return make(map[MetadataKey]any)
}

// Has checks if metadata exists for a type and key
func (ms *MetadataStorage) Has(target any, key MetadataKey) bool {
	_, exists := ms.Get(target, key)
	return exists
}

// Delete removes metadata for a type and key
func (ms *MetadataStorage) Delete(target any, key MetadataKey) {
	t := reflect.TypeOf(target)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	if typeData, exists := ms.data[t]; exists {
		delete(typeData, key)
	}
}

// Clear removes all metadata for a type
func (ms *MetadataStorage) Clear(target any) {
	t := reflect.TypeOf(target)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	delete(ms.data, t)
}

// RouteMetadata stores route-specific metadata
type RouteMetadata struct {
	Method      string
	Path        string
	Summary     string
	Description string
	Tags        []string
	Deprecated  bool
}

// ControllerMetadata stores controller-specific metadata
type ControllerMetadata struct {
	Path        string
	Tags        []string
	Description string
}

// ParameterMetadata stores parameter-specific metadata
type ParameterMetadata struct {
	Name        string
	Type        string // "query", "path", "body", "header"
	Required    bool
	Description string
	Schema      any
}
