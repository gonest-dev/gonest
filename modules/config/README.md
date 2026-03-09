# GoNest Config Module

Advanced configuration management for GoNest, based on environment variables and `.env` files.

## Features

- **Typed Access**: Retrieve configuration values as strings, integers, booleans, or custom types.
- **Variable Expansion**: Support for `${VAR}` syntax in `.env` files.
- **Dependency Injection**: Use `ConfigService` in any of your providers.
- **Multiple Environments**: Load different `.env` files for different environments.

## Usage

Register the `ConfigModule` in your root module:

```go
import (
    "github.com/gonest-dev/gonest/modules/config"
    "github.com/gonest-dev/gonest/core/common"
)

@common.Module({
    Imports: []any{
        config.ForRoot(".env", ".env.local"),
    },
})
type AppModule struct{}
```

Then inject `ConfigService` in your providers or controllers:

```go
type MyService struct {
    Config *config.ConfigService `inject:""`
}

func (s *MyService) DoSomething() {
    port := config.GetTyped[int](s.Config, "PORT", 8080)
    // ...
}
```
