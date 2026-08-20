> src/domain/port/port.go
```go
type Pingable interface {
  Ping(context.Context) error
}
```

> src/infra/adapter/sql/adapter/adapter.go
```go
import "src/domain/port"

type Adapter struct {}
var _ port.Pingable = (*Adapter)(nil)
func (s *Adapter) Ping(ctx context.Context) error { ... }
var Adapter_ = gonest.NewProvider(func (p gonest.Provider) {
  p.Constructor(func() *Adapter { return &Adapter{} })
})
var Pingable_ = gonest.ProviderAs[port.Pingable](Adapter_)
```

> src/infra/adapter/sql/module.go
```go
var Module = gonest.NewModule(func (m gonest.Module) {
  m.Providers(adapter.Adapter_)
  m.Exports(adapter.Pingable_)
})
```

> src/infra/adapter/cache/adapter/adapter.go
```go
import "src/domain/port"
type Adapter struct{}
var _ port.Pingable = (*Adapter)(nil)
func (s *Adapter) Ping(ctx context.Context) error { ... }
var Adapter_ = gonest.NewProvider(func (p gonest.Provider) {
  p.Constructor(func() *Adapter { return &Adapter{} })
})
var Pingable_ = gonest.ProviderAs[port.Pingable](Adapter_)
```

> src/infra/adapter/cache/module.go
```go
var Module_ = gonest.NewModule(func (m gonest.Module) {
  m.Providers(adapter.Adapter_)
  m.Exports(adapter.Pingable_)
})
```

> src/infra/adapter/broker/adapter/adapter.go
```go
import "src/domain/port"
type Adapter struct{}
var _ port.Pingable = (*Adapter)(nil)
func (s *Adapter) Ping(ctx context.Context) error { ... }
var Adapter_ = gonest.NewProvider(func (p gonest.Provider) {
  p.Constructor(func() *Adapter { return &Adapter{} })
})
var Pingable_ = gonest.ProviderAs[port.Pingable](Adapter_)
```

> src/infra/adapter/broker/module.go
```go
var Module_ = gonest.NewModule(func (m gonest.Module) {
  m.Providers(adapter.Adapter_)
  m.Exports(adapter.Pingable_)
})
```

> src/app/system/usecase/health/health.go
```go
type Usecase struct { pingables []port.Pingable }
func (u *Usecase) Execute(ctx context.Context) error { ... }
var Usecase_ = gonest.NewProvider(func (p gonest.Provider) {
  pingables := gonest.MustInjectAll[port.Pingable](p)
  p.Constructor(func () *Usecase { return &Usecase{ pingables: pingables } })
})
```

> src/app/system/module.go
```go
var Module = gonest.NewModule(func (m gonest.Module) {
  m.Imports(adapter.Module, adapter.Module_, adapter.Module_)
  m.Providers(usecase.Usecase_)
})
```