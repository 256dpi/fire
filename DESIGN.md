# Design

This document describes some of of the design patterns used throughout the fire
framework.

## Classifiable Models

**The classifiable models pattern is used to provide an idiomatic way of working
with user definable data types. It builds on the builtin Go struct tagging
mechanism and formalizes a method that is both extendable and type-safe.**

A framework can create a class of structs be defining a `Model` interface that
can be implemented by embedding a provided `Base` type in a struct. The interface
must at least require the `GetBase` method that is implemented automatically when
embedding `Base`. Additional methods may added to the interface to require more
functionality.

The package level `GetMeta` function can be used by the framework or application
to classify a type and retrieve its `Meta` object. The `Meta` object contains
all information that can be deducted from the type using reflection. Tags on the
embedded `Base` type describe the type itself, while tags on the struct fields
add field level information. The custom tag should be named after the package.

The `Meta` might contain additional fields that inform and control the framework
and application code. These fields should have sensible default values that only
need to be changed in advanced scenarios. The defaults may be changed directly
on the instance returned by `GetMeta`.

If a package uses multiple classifiable models it should add the class name to
exported types and functions e.g. `ClassBase`, `GetClassBase`, `ClassMeta` and
`GetClassMeta`. However, it is recommended to only use one class per package.

### Framework Code

```go
package pkg

// Model is a classifiable model.
type Model interface {
    GetBase() *Base
    SomeMethod() string
}

// The Base struct is embedded in other structs as the first field to make them
// compatible with the Model interface.
type Base struct {
    SomeField string
}

// GetBase implements the Model interface.
func (b *Base) GetBase() *Base {
    return b
}

// The Meta struct is returned by GetMeta.
type Meta struct {
    SomeField string
}

// GetMeta will analyze the provided model and return information about it.
func GetMeta(model Model) *Meta {
    return &Meta{}
}
```

### Application Code

```go
package app

var _ pkg.Model = &Entity{}

// Entity is an example entity that embeds the Base struct and implements Method
// do be Model compatible.
type Entity struct {
    pkg.Base `pkg:"foo,bar"`

    SomeField string
}

// SomeMethod implements the Model interface.
func (e *Entity) SomeMethod() string {
    return e.Base.SomeField + ", " + e.SomeField
}
```

## Open Controllers

**The open controllers pattern is used to abstract common logic and provide an
open surface to configure its execution.**

The `Controller` type is a type that is instantiated by the user to configure a
single unit of logic abstraction. The constructor less design allows the adding
of more knobs and switches in the future without generating churn. The controller
instances are then provided to the `Manager` that provides little configuration
and is created using an constructor. While the `Controller` has no public methods
the `Manager` provides public methods to control the execution of the logic.

The open controllers pattern may be combined with the classifiable models
pattern to allow customization of the abstracted logic's inner data structure.

### Framework Code

```go
package pkg

type Controller struct {
    SomeField string
    SomeModel Model
}

func (c *Controller) execute() error {
    // ...
    return nil
}

// Controller has no public methods.

type Manager struct{
    // no public fields
}

func NewManager() *Manager {
    return &Manager{}
}

func (m *Manager) Add(c *Controller) {
    // add controller
}

func (m *Manager) Run() error {
    // ...
    return nil
}
```

### Application Code

```go
package app

func Run() {
    // create manager
    manager := pkg.NewManager()

    // add controller
    manager.Add(&pkg.Controller{
        SomeField: "foo",
        SomeModel: &Entity{},
    })

    // run manager
    manager.Run()
}
```

## Rich Contexts

**The rich contexts pattern is an alternative to Go's `context.Context` API
which tries to add more type safety instead of `interface{}` juggling.**

The framework provides a custom `Context` type that embeds a `context.Context`
for compatibility but only uses it to carry the externally provided context.
All framework related fields are openly accessible on the context. Ths context
is the passed to all called user handlers.

### Framework Code

```go
package pkg

import "context"

type Context struct{
    context.Context

    SomeField      string
    SomeModel      Model
    SomeController *Controller
    SomeManager    *Manager
}
```

## Function Handlers

**The function handlers pattern promotes the use of function handlers instead
of interface types to integrate custom logic. At best it is combined with 
patterns like open controllers and rich contexts.**

The framework may need to integrate custom logic that is run as part of a more
complex abstracted logic. Instead of requiring an interface type the framework
allows the configuration of handlers. The openness of the handlers allows them
to be easily generated or curried (wrapped) with other functions. 

### Framework Code

```go
package pkg

type Context struct {}

type Controller struct{
    Handler1 func(*Context) error
    Handler2 func(*Context) error
    Handler3 func(*Context) error
}
```

### Application Code

```go
package app

func ExampleController() *pkg.Controller {
    return &pkg.Controller{
        Handler1: func(ctx *pkg.Context) error {
            // ...
            return nil
        },
        Handler2: makeHandler(),
        Handler3: wrapHandler(func(ctx *pkg.Context) error {
            // ...
            return nil
        }),
    }
}
``` 
