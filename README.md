# GoNest Framework

<p align="center">
  <a href="https://gonest.dev/" target="blank">
    <img src=".public/icon.svg" width="120" alt="GoNest Logo">
  </a>
</p>

A NestJS-inspired framework for **Go** designed for building efficient, reliable, and scalable server-side applications. It leverages Go's performance and type safety while providing a familiar architectural pattern for developers coming from the NestJS ecosystem.

## 🚀 Overview

GoNest provides an out-of-the-box application architecture which allows developers and teams to create highly testable, scalable, loosely coupled, and easily maintainable applications. It combines the power of Go's concurrency with advanced Dependency Injection and a metadata-driven API.

## ✨ Features

* **Advanced Dependency Injection:** Fully featured DI container supporting Singleton, Transient, and Request scopes.
* **Type-Safe Validation:** Built-in validator core with 86+ rules across 9 categories (String, Number, Date, Array, etc.).
* **Decorator-inspired API:** Controller and Routing system using struct tags and reflection to mimic the NestJS developer experience.
* **Modular Architecture:** Easily organize code into modules with circular dependency detection.
* **Context System:** Integrated request context with middleware chain support.
* **Lifecycle Hooks:** Manage application stages with `OnModuleInit`, `OnApplicationBootstrap`, and more.

## 🛠️ Quick Start

```go
package main

import (
	"github.com/gonest-dev/gonest/core"
	"github.com/gonest-dev/gonest/common"
)

type AppController struct {}

func (c *AppController) GetHello() string {
	return "Hello GoNest!"
}

func main() {
	app := core.Create(AppModule)
	app.Listen(3000)
}

```

## 📈 Roadmap & Status

GoNest is currently in **v0.1.0 Alpha** (March 2026).

### Completed Milestones

* ✅ **Phase 1: Foundation** (Core Architecture, DI, Context)
* ✅ **Phase 2: Type-Safe Validation** (86+ rules, Async support)
* ✅ **Phase 3: Decorators & Routing** (Controller system, Pipes)

### Upcoming

* 🚧 **Phase 4: Guards & Security** (Auth, JWT, RBAC)
* 🚧 **Phase 5: Interceptors & Middleware**
* 🚧 **Phase 7: Swagger/OpenAPI Integration**

## 🤝 Contributing

Contributions are welcome! Please check our [Contributing Guide](https://www.google.com/search?q=./CONTRIBUTING.md) and the [Roadmap](https://www.google.com/search?q=./ROADMAP.md) for current priorities.

## 🔗 Links

* **Website:** [gonest.dev](https://www.google.com/search?q=https://gonest.dev)
* **Discord:** [Join Community](https://discord.gg/gonest)
* **License:** [MIT](https://www.google.com/search?q=./LICENSE)

---

Developed with ❤️ by [gonest team](https://www.google.com/search?q=https://gonest.dev)

Would you like me to help you draft the `CONTRIBUTING.md` or a specific technical section for the `Documentation` mentioned in your roadmap?