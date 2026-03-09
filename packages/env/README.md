# GoNest Env

Lightweight, high-performance environment variable loader and parser for Go. Ported from `gox/env`.

## Features

- **Typed Access**: Generic `Get[T]` function supporting basic types, slices, JSON, `time.Time`, and `time.Duration`.
- **.env Support**: Loads and parses `.env` files with support for quotes, comments, and `export`.
- **Variable Expansion**: Supports `${VAR}` or `$VAR` interpolation.
- **Thread-safe**: Safe for concurrent reads and writes.

## Usage

```go
import "github.com/gonest-dev/gonest/core/env"

func main() {
    // Load .env file
    env.Load(".env")

    // Get typed variable with default
    port := env.Get[int]("PORT", 8080)
    
    // Expand variables
    dbURL := env.Get[string]("DATABASE_URL")
}
```
