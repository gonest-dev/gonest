```go
package ex

import "gonest.dev/gonest"

type DbService struct {}
func (* DbService) Connect() error { /* ... */  }
func (* DbService) Close() error { /* ... */  }
func (* DbService) Ping() error { /* ... */  }

var DbProvider = gonest.NewProvider(func (p *gonest.Provider) {
  p.Constructor(func () DbService { return &DbService{} })
  p.OnApplicationBootstrap(func (s *DbService) error { return s.Connect() })
  p.OnApplicationShutdown(func (s *DbService) error { return s.Close() })
  p.OnModuleInit(func (s *DbService) error { return s.Ping() })
  p.OnModuleDestroy(func (s *DbService) error { return nil })
})
```