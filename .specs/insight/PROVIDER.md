# Hipótese sobre reformulação estrutural para providers e talvez para outras estruturas

A estrutura atual para declaração dos providers não está me deixando feliz de forma que além de não ser consistente 
ainda não permite a inferência de uma interface nos providers. Com isso a inteção desse arquivo é tentar garimpar sobre
as possibilidades do golang pra tentar chegar em um padrão para o framework que mesmo que vá contra os princípios do 
golang mas ainda tenha uma assinatura simples e amigável como o verdadeiro nestjs faz.

> **Status:** SHIPPED (Milestone 22 completo -- `gonest.ProviderAs[Interface](ref)`
> explícito é código real em `gonest.go`, fallback estrutural `Implements()` removido
> de `internal/resolver/direct.go`; Milestone 23 fechou a convenção `Thing_` pra vars
> de builder, formalizada em README.md, e migrou `.examples/notification-driver` --
> ver `.specs/features/provider-interface-export/{spec,tasks}.md`). O
> `Module.Lazy`/`l.Imports(...)` do exemplo abaixo (`database/module.go`) mirava
> replicar o `DynamicModule.forRootAsync` do NestJS -- SHIPPED (Milestone 24
> completo: `gonest.LazyModule`/`(*Module).Lazy` são código real, não mais
> hipótese; `.examples/notification-driver` migrado de `ModuleForRoot(driver)`
> pra `notifier.Config_` + `m.Lazy(...)` -- ver
> `.specs/features/module-lazy-loading/{spec,design,tasks}.md`).

## exemplo de declaração de um provider simples com registro em módulo

> estrutura geral
```
src/
  core/must.go
  domain/
    entity/
      indexable.go
      creatable.go
      updatable.go
      deletable.go
      person.go
    repository/person.go
  infra/
    database/
      impl/
        memory/
          repository/person.go
          module.go
        postgres/
          config/config.go
          service/service.go
          repository/person.go
          module.go
      config/config.go
      module.go
```

> src/core/must.go
```go
package core

func MustValue[T any](value T, err error) T {
  if err != nil {
    panic(err)
  }
  return value
}

func MustReturn(err error) {
  if err !=nil {
    panic(err)
  }
}
```

> src/domain/entity/indexable.go
```go
package entity

import (
  "time"
  _sc "src/core"
  "github.com/google/uuid"
)

type Indexable struct { 
  ID string `json:"id"` 
}
func NewIndexable() *Indexable {
  return &Indexable{ID: _sc.MustValue(uuid.NewV7()).String()}
}
```

> src/domain/entity/creatable.go
```go
package entity

import "time"

type Creatable struct { 
  CreatedAt time.Time `json:"created_at"` 
}
func NewCreatable() *Creatable {
  return &Creatable{CreatedAt: time.Now()}
}
```

> src/domain/entity/updatable.go
```go
package entity

import "time"

type Updatable struct { 
  UpdatedAt time.Time `json:"updated_at"` 
}
func NewUpdatable() *Updatable {
  return &Updatable{UpdatedAt: time.Now()}
}
```

> src/domain/entity/deletable.go
```go
package entity

import "time"

type Deletable struct { 
  DeletedAt *time.Time `json:"deleted_at"` 
}
func NewDeletable() *Deletable {
  return &Deletable{DeletedAt: nil}
}
```

> src/domain/entity/person.go
```go
package entity

import (
  "time"
  "gonest.dev/gonest"
)

type Person_Props struct {
  Name      gonest.Accessor[string]    `json:"name"`
  Age       gonest.Accessor[int]       `json:"age"`
  IsActive  gonest.Accessor[bool]      `json:"is_active"`
  BirthDate gonest.Accessor[time.Time] `json:"birth_date"`
}
func NewPerson_Props(p ...*Person_Props) *Person_Props {
  i := Person_Props{
    Name: gonest.NewAccessor(),
    Age:  gonest.NewAccessor(),
    IsActive:  gonest.NewAccessor(false),
    BirthDate: gonest.NewAccessor(),
  }
  if len(p) > 0 && p[0] != nil {
    &p[0].Name.Sync(i.Name)
    &p[0].Age.Sync(i.Age)
    &p[0].IsActive.Sync(i.IsActive)
    &p[0].BirthDate.Sync(i.BirthDate)
  }
  return &i
}

type Person struct {
	Indexable
  Creatable
  Updatable
  Deletable
	Person_Props
}
func NewPerson(p ...*Person_Props) *Person {
	return &Person{
    Indexable: *NewIndexable(),
    Creatable: *NewCreatable(),
    Updatable: *NewUpdatable(),
    Deletable: *NewDeletable(),
    Person_Props: *NewPerson_Props(p...),
  }
}
```

> src/domain/repository/person.go
```go
package repository

import (
  "context"
  _sde "src/domain/entity"
)

type Person interface {
  List(ctx context.Context) ([]*_de.Person, error)
  Get(ctx context.Context, id string) (*_de.Person, error)
  Create(ctx context.Context, props *_de.Person_Props) (*_de.Person, error)
  Update(ctx context.Context, id string, props *_de.Person_Props) (*_de.Person, error)
  Delete(ctx context.Context, id string) (*_de.Person, error)
}
```

> src/infra/database/impl/memory/repository/person.go
```go
package repository

import (
  "context"
	"time"
  _sde "src/domain/entity"
  _sre "src/domain/repository"
	"gonest.dev/gonest"
)

type Person struct {
  items  []*_sde.Person
}

var _ _sre.Person = (*Person)(nil)

func (r *Person) List(_ context.Context) ([]*_sde.Person, error) {
  return r.items, nil
}

func (r *Person) Get(_ context.Context, id string) (*_sde.Person, error) {
  for _, i := range r.items {
    if i.ID == id { 
      return &i, nil 
    }
  }
  return nil, nil
}

func (r *Person) Create(_ context.Context, props *_sde.Person_Props) (*_sde.Person, error) {
  r.items = append(r.items, _sde.NewPerson(props))
  return r.items[len(r.items)-1], nil
}

func (r *Person) Update(_ context.Context, id string, props *_sde.Person_Props) (*_sde.Person, error) {
  item, err := r.Get(id)
  if err != nil {
    return nil, err
  }
  if item != nil {
    item.Person_Props = *_sde.NewPerson_Props(props)
    return item, nil
  }
  return nil, nil
}

func (r *Person) Delete(_ context.Context, id string) (*_sde.Person, error) {
  for i, item := range r.items {
    if item.ID == id {
      r.items = append(r.items[:i], r.items[i+1:]...)
      return item, nil
    }
  }
  return nil, nil
}

var Person_ = gonest.NewProvider(func(p *gonest.Provider) {
  p.Constructor(func() *Person { 
    return &Person{
      items: make([]*_sde.Person, 0),
    } 
  })
})
```

> src/infra/database/impl/memory/module.go
```go
package memory

import (
  _sre "src/domain/repository"
  _idimr "src/infra/database/impl/memory/repository"
  "gonest.dev/gonest"
)

var Module_ = gonest.NewModule(func (m *gonest.Module) {
  m.Providers(
    _idimr.Person_,
    gonest.ProviderAs[_sre.Person](_idimr.Person_),
  )
})
```

> src/infra/database/impl/postgres/config/config.go
```go
package config

import "gonest.dev/gonest"

type Database struct {
  DSN string `json:"dsn"`
}

var Database_ = gonest.NewSchema(func (t *Database, s *gonest.ObjectSchema) {
  s.Property(&t.DSN).String().Required()
})
```

> src/infra/database/impl/postgres/service/service.go
```go
package service

import (
  _sc "src/core"
  _sidipc "src/infra/database/impl/postgres/config"
  "gonest.dev/gonest"
  "github.com/leandroluk/golem"
  "github.com/leandroluk/golem/driver/postgres"
)

type Database struct {
  gonest.DataSource
}

var Database_ = gonest.NewProvider(func(p *gonest.Provider) {
  p.Constructor(func() *Database {
		c := gonest.MustInject[*_sidipc.Database](p)
		return &Database{
			DataSource: *golem.MustNewDataSource(
        postgres.New(func(o *postgres.Options) { o.DSN = c.DSN }),
      ),
		}
	})
	p.OnApplicationBootstrap(func(d *Database) { _sc.MustReturn(d.Connect())	})
	p.OnApplicationShutdown(func(d *Database) { _sc.MustReturn(d.Close()) })
})
```

> src/infra/database/impl/postgres/repository/person.go
```go
package repository

import (
	"time"
  _sde "src/domain/entity"
  _sre "src/domain/repository"
  _sidips "src/infra/database/impl/postgres/service"
	"gonest.dev/gonest"
  "github.com/leandroluk/golem"
)

type Person struct {
  repository *golem.Repository[_sde.Person]
}

var _ _sre.Person = (*Person)(nil)

func (r *Person) List(ctx context.Context) ([]*_sde.Person, error) {
  items, err := r.repository.FindMany(ctx)
  return r.items, nil
}

func (r *Person) Get(ctx context.Context, id string) (*_sde.Person, error) {
  return r.repository.FindOne(ctx, func(a *_sde.Person, q *golem.Query[_sde.Person]) {
		q.Where(golem.Eq(&a.ID, id))
	})
}

func (r *Person) Create(ctx context.Context, props *_sde.Person_Props) (*_sde.Person, error) {
  return r.repository.Insert(ctx, _sde.NewPerson(props))
}

func (r *Person) Update(ctx context.Context, id string, props *_sde.Person_Props) (*_sde.Person, error) {
  item, err := r.repository.FindOne(ctx, func(a *_sde.Person, q *golem.Query[_sde.Person]) {
		q.Where(golem.Eq(&a.ID, id))
	})
	if err != nil {
		return nil, err
	}
	item.Person_Props = *_sde.NewPerson_Props(props)
	return r.repository.SaveOne(ctx, item)
}

func (r *Person) Delete(ctx context.Context, id string) (*_sde.Person, error) {
  return r.repository.Delete(ctx, func(a *_sde.Person, q *golem.Query[_sde.Person]) {
		q.Where(golem.Eq(&a.ID, id))
	})
}

var Person_ = gonest.NewProvider(func(p *gonest.Provider) {
  database := gonest.MustInject[*_sidips.Database](p)
  var table = golem.NewTable(func(t *_sde.Person, b *golem.Table) {
    b.TableName("tb_person")
    b.Col(&t.ID, golem.UUID()).Name("id")
    b.Col(&t.CreatedAt, golem.DATETIME()).Name("created_at")
    b.Col(&t.UpdatedAt, golem.DATETIME()).Name("updated_at")
    b.Col(&t.DisabledAt, golem.DATETIME()).Name("disabled_at")
    b.Col(&t.Name, golem.TEXT()).Name("name")
    b.Col(&t.Age, golem.INTEGER()).Name("age")
    b.Col(&t.IsActive, golem.BOOLEAN()).Name("is_active")
    b.Col(&t.BirthDate, golem.DATE()).Name("birth_date")

    b.PrimaryKey(&t.ID)
    b.CreateDate(&t.CreatedAt)
    b.UpdateDate(&t.UpdatedAt)
    b.DeleteDate(&t.DisabledAt)
  })
	p.Constructor(func() *Person {
		return &Person{repository: golem.NewRepository(database, table)}
	})
})
```

> src/infra/database/impl/postgres/module.go
```go
package postgres

import (
  _sdr "src/domain/repository"
  _sidipc "src/infra/database/impl/postgres/config"
  _sidipr "src/infra/database/impl/postgres/repository"
  _sidips "src/infra/database/impl/postgres/service"
  "gonest.dev/gonest"
)

var Module_ = gonest.NewModule(func (m *gonest.Module) {
  m.Providers(
    _sidipc.Database_,
    _sidipr.Person_,
    _sidips.Database_,
    gonest.ProviderAs[_sdr.Person](_sidipr.Person_),
  )
})
```

> src/infra/database/config/config.go
```go
package config

import "gonest.dev/gonest"

type Config struct { 
  Provider string `json:"provider"`
}

var Config_ = gonest.NewSchema(func (t *Config, s *gonest.ObjectSchema) {
  s.Property(&t.Provider).String().Enum("memory", "fake").Default("memory")
})
```

> src/infra/database/module.go
```go
package database

import (
  "gonest.dev/gonest"
  _sidim "src/infra/database/impl/memory"
  _sidip "src/infra/database/impl/postgres"
)

var Module_ = gonest.NewModule(func (m *gonest.Module) {
  m.Providers(Config_)
  m.Lazy(func (l *gonest.LazyModule) {
    config := gonest.MustInject[*Config](l)
    switch config.Provider {
    case "memory":
      l.Imports(_sidim.Module_)
      l.Exports(_sidim.Module_)
    case "postgres":
      l.Imports(_sidip.Module_)
      l.Exports(_sidip.Module_)
    default:
      panic("Invalid database provider: " + config.Provider)
    }
  })
})
```

