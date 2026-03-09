// gonest/core/controller/params.go
package controller

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gonest-dev/gonest/core/common"
)

// ParamExtractor extracts parameters from context based on configuration
type ParamExtractor struct{}

// NewParamExtractor creates a new parameter extractor
func NewParamExtractor() *ParamExtractor {
	return &ParamExtractor{}
}

// Extract extracts a parameter from context based on config
func (pe *ParamExtractor) Extract(ctx *common.Context, config *ParamConfig) (any, error) {
	switch config.Type {
	case ParamTypeBODY:
		return pe.extractBody(ctx, config)
	case ParamTypeQUERY:
		return pe.extractQuery(ctx, config)
	case ParamTypePARAM:
		return pe.extractParam(ctx, config)
	case ParamTypeHEADER:
		return pe.extractHeader(ctx, config)
	case ParamTypeREQ:
		return ctx, nil
	case ParamTypeRES:
		return ctx, nil
	default:
		return nil, fmt.Errorf("unsupported param type: %s", config.Type)
	}
}

// extractBody extracts body parameter
func (pe *ParamExtractor) extractBody(ctx *common.Context, config *ParamConfig) (any, error) {
	var body map[string]any

	if err := ctx.BindJSON(&body); err != nil {
		if config.Required {
			return nil, fmt.Errorf("failed to parse body: %w", err)
		}
		return nil, nil
	}

	if config.Name != "" {
		value, exists := body[config.Name]
		if !exists && config.Required {
			return nil, fmt.Errorf("body parameter '%s' is required", config.Name)
		}
		return value, nil
	}

	return body, nil
}

// extractQuery extracts query parameter
func (pe *ParamExtractor) extractQuery(ctx *common.Context, config *ParamConfig) (any, error) {
	value := ctx.Query(config.Name)

	if value == "" && config.Required {
		return nil, fmt.Errorf("query parameter '%s' is required", config.Name)
	}

	if config.Transform != nil {
		return config.Transform(value)
	}

	return value, nil
}

// extractParam extracts path parameter
func (pe *ParamExtractor) extractParam(ctx *common.Context, config *ParamConfig) (any, error) {
	value := ctx.Param(config.Name)

	if value == "" && config.Required {
		return nil, fmt.Errorf("path parameter '%s' is required", config.Name)
	}

	if config.Transform != nil {
		return config.Transform(value)
	}

	return value, nil
}

// extractHeader extracts header parameter
func (pe *ParamExtractor) extractHeader(ctx *common.Context, config *ParamConfig) (any, error) {
	value := ctx.Get(config.Name)

	if value == "" && config.Required {
		return nil, fmt.Errorf("header '%s' is required", config.Name)
	}

	return value, nil
}

// ParseInt creates a transform function that parses string to int
func ParseInt() func(any) (any, error) {
	return func(value any) (any, error) {
		str, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", value)
		}

		return strconv.Atoi(str)
	}
}

// ParseFloat creates a transform function that parses string to float64
func ParseFloat() func(any) (any, error) {
	return func(value any) (any, error) {
		str, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", value)
		}

		return strconv.ParseFloat(str, 64)
	}
}

// ParseBool creates a transform function that parses string to bool
func ParseBool() func(any) (any, error) {
	return func(value any) (any, error) {
		str, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", value)
		}

		return strconv.ParseBool(str)
	}
}

// BodyDTO creates a body parameter that binds to a DTO
func BodyDTO[T any]() *ParamConfig {
	return &ParamConfig{
		Type:     ParamTypeBODY,
		Name:     "",
		Required: true,
		Transform: func(value any) (any, error) {
			// Convert to JSON and back to bind to type T
			data, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}

			var result T
			if err := json.Unmarshal(data, &result); err != nil {
				return nil, err
			}

			return result, nil
		},
	}
}


