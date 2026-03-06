// gonest/interceptors/transform_interceptor.go
package interceptors

import (
	"encoding/json"
)

// TransformInterceptor transforms responses
type TransformInterceptor struct {
	transform func(any) (any, error)
}

// NewTransformInterceptor creates a new transform interceptor
func NewTransformInterceptor(transform func(any) (any, error)) *TransformInterceptor {
	return &TransformInterceptor{
		transform: transform,
	}
}

// Intercept transforms the response
func (i *TransformInterceptor) Intercept(ctx *ExecutionContext, next func() error) error {
	// Execute handler
	err := next()
	if err != nil {
		return err
	}

	// Get response from context (if set)
	response := ctx.Context.Get("response")
	if response != nil && i.transform != nil {
		transformed, err := i.transform(response)
		if err != nil {
			return err
		}
		ctx.Context.Set("response", transformed)
	}

	return nil
}

// WrapResponse wraps all responses in a standard format
func WrapResponse() *TransformInterceptor {
	return NewTransformInterceptor(func(data any) (any, error) {
		return map[string]any{
			"success": true,
			"data":    data,
		}, nil
	})
}

// AddMetadata adds metadata to responses
func AddMetadata(metadata map[string]any) *TransformInterceptor {
	return NewTransformInterceptor(func(data any) (any, error) {
		// If data is already a map, add metadata to it
		if dataMap, ok := data.(map[string]any); ok {
			for key, value := range metadata {
				dataMap[key] = value
			}
			return dataMap, nil
		}

		// Otherwise, wrap data with metadata
		result := map[string]any{
			"data": data,
		}
		for key, value := range metadata {
			result[key] = value
		}
		return result, nil
	})
}

// SerializeResponse ensures response is properly serialized
func SerializeResponse() *TransformInterceptor {
	return NewTransformInterceptor(func(data any) (any, error) {
		// Serialize and deserialize to ensure clean JSON
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}

		var result any
		if err := json.Unmarshal(jsonData, &result); err != nil {
			return nil, err
		}

		return result, nil
	})
}
