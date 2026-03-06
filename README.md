<div align="center">

# GoNest Framework

<img src=".public/icon.svg" alt="GoNest Logo" width="200"/>

**A NestJS-inspired framework for Go with complete type-safety**

[![CI](https://github.com/gonest-dev/gonest/workflows/ci/badge.svg)](https://github.com/gonest-dev/gonest/actions)
[![Coverage](https://raw.githubusercontent.com/gonest-dev/gonest/main/.public/coverage.svg)](https://github.com/gonest-dev/gonest)
[![Go Version](https://img.shields.io/badge/go-1.23-blue.svg)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[Documentation](https://gonest.dev) • [Workspace](https://github.com/gonest-dev/workspace) • [Examples](https://github.com/gonest-dev/gonest-examples)

</div>

---

## 🚀 Quick Start

```bash
go get github.com/gonest-dev/gonest
```

```go
package main

import (
    "github.com/gonest-dev/gonest/core"
    "github.com/gonest-dev/gonest/controller"
)

func main() {
    app := core.NewApplication()

    ctrl := controller.NewController(
        controller.WithPrefix("/"),
    )

    ctrl.Get("/", func(ctx *core.Context) error {
        return ctx.JSON(200, map[string]any{
            "message": "Hello, GoNest!",
        })
    })

    app.RegisterController(ctrl)
    app.Listen(":3000")
}
```

---

## 📦 Modules

| Module                         | Description            | Coverage                                                                                                |
| ------------------------------ | ---------------------- | ------------------------------------------------------------------------------------------------------- |
| [core](./core)                 | DI, Modules, Lifecycle | ![Coverage](https://raw.githubusercontent.com/gonest-dev/gonest/main/.public/core-coverage.svg)         |
| [validator](./validator)       | 86+ validation rules   | ![Coverage](https://raw.githubusercontent.com/gonest-dev/gonest/main/.public/validator-coverage.svg)    |
| [controller](./controller)     | HTTP routing           | ![Coverage](https://raw.githubusercontent.com/gonest-dev/gonest/main/.public/controller-coverage.svg)   |
| [pipes](./pipes)               | Data transformation    | ![Coverage](https://raw.githubusercontent.com/gonest-dev/gonest/main/.public/pipes-coverage.svg)        |
| [guards](./guards)             | Security & auth        | ![Coverage](https://raw.githubusercontent.com/gonest-dev/gonest/main/.public/guards-coverage.svg)       |
| [interceptors](./interceptors) | Request/response       | ![Coverage](https://raw.githubusercontent.com/gonest-dev/gonest/main/.public/interceptors-coverage.svg) |
| [exceptions](./exceptions)     | Error handling         | ![Coverage](https://raw.githubusercontent.com/gonest-dev/gonest/main/.public/exceptions-coverage.svg)   |
| [swagger](./swagger)           | OpenAPI 3.0.3          | ![Coverage](https://raw.githubusercontent.com/gonest-dev/gonest/main/.public/swagger-coverage.svg)      |
| [adapters](./adapters)         | Platform support       | ![Coverage](https://raw.githubusercontent.com/gonest-dev/gonest/main/.public/adapters-coverage.svg)     |

---

## ✨ Features

- 🎯 **Type-Safe** - Full compile-time checking with generics
- 🔒 **Secure** - Built-in guards and authentication
- 📝 **Validated** - 86+ validation rules
- 📖 **Documented** - Auto-generated OpenAPI docs
- 🚀 **Fast** - Zero reflection in hot paths
- 🔌 **Platform Agnostic** - Gin, Fiber, Echo, Chi, Mux
- 🧩 **Modular** - Clean architecture with DI
- 🎨 **Familiar** - API inspired by NestJS

---

## 📚 Documentation

- 📖 [Full Documentation](https://gonest.dev)
- 🚀 [Getting Started](https://gonest.dev/getting-started)
- 📘 [Core Concepts](https://gonest.dev/core-concepts)
- 🎯 [API Reference](https://gonest.dev/api)

---

## 🛠️ Development

### Prerequisites

- Go 1.23+
- Make
- [gonest-tools](https://github.com/gonest-dev/gonest-tools) (optional, for badges and tags)

### Setup

```bash
git clone https://github.com/gonest-dev/gonest.git
cd gonest
```

### Commands

```bash
# Testing
make test              # Run tests
make coverage          # Generate coverage
make ci                # Test + coverage + badges

# Building
make build             # Build all modules
make lint              # Run linter

# Development
make format            # Format code
make mod-tidy          # Tidy modules

# Cleanup
make clean             # Remove artifacts
```

### Installing gonest-tools

For badge generation and tag management:

```bash
# Clone tools repo
git clone https://github.com/gonest-dev/gonest-tools.git
cd gonest-tools

# Install globally
make install

# Now you can use in gonest repo:
cd ../gonest
make badges            # Generate coverage badges
make tag v0.1.0        # Create and push tags
```

---

## 🏷️ Release Management

Using [gonest-tag](https://github.com/gonest-dev/gonest-tools):

```bash
# Create and push tags for all modules
make tag v0.1.0

# Bump patch (0.1.0 -> 0.1.1)
make tag-minor

# Bump minor (0.1.0 -> 0.2.0)
make tag-major
```

This creates tags for:
- `v0.1.0` (root)
- `core/v0.1.0`
- `validator/v0.1.0`
- `controller/v0.1.0`
- ... (all modules)

---

## 🤝 Contributing

See [CONTRIBUTING.md](https://github.com/gonest-dev/workspace/blob/main/CONTRIBUTING.md) in the workspace repo.

---

## 📜 License

[MIT](LICENSE) © 2024 GoNest Contributors

---

## 🔗 Related Repositories

- [workspace](https://github.com/gonest-dev/workspace) - Monorepo
- [gonest-tools](https://github.com/gonest-dev/gonest-tools) - Development tools
- [gonest-examples](https://github.com/gonest-dev/gonest-examples) - Example apps
- [gonest-docs](https://github.com/gonest-dev/gonest-docs) - Documentation

---

<div align="center">

**Made with ❤️ by the GoNest team**

</div>
