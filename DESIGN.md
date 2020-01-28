# Design

This document describes some of of the design patterns used throughout the fire
framework.

## Classifiable Models

**The classifiable models pattern is used to provide an idiomatic way of working
with user definable data types. It builds on the powerful builtin Go struct
tagging system and formalizes a usage that is as open and as type-safe as
possible at the same time.**

Usually a framework expects a certain class of structs that for example
represent database models. To define this class of structs the framework should
define a `Model` interface that can be implemented by simply embedding a `Base`
type in a struct. This is achieved by the private `base` method that just returns
the base type. Additional methods may added to the interface to require more
functionality. The `Classify` function can then be used by the framework itself
or the user to classify a type and get its `Meta` object that contains all
information that can be deducted from the type using reflection. Usually tags on
the embedded `Base` type describe the type itself, like its name.

### Framework Code

```go
package pkg

// Model defines a classifiable set of structs.
type Model interface {
	Method() string
	GetBase() *Base
}

// The Base struct is embedded in other structs as the first field to mark them
// as compatible with the Model interface.
type Base struct {
	Field string
}

// GetBase implements the Model interface.
func (b *Base) GetBase() *Base {
	return b
}

// The Meta struct is returned by Classify.
type Meta struct {
	Field string
}

// Classify will analyze the provided model and return information about its
// class by analyzing the types struct fields and tags.
func Classify(model Model) Meta {
	return Meta{}
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

	Field string
}

// Method implements the Model interface.
func (e *Entity) Method() string {
	return e.Base.Field + ": " + e.Field
}
```

## Open Controllers

**The open controllers pattern is used to abstract common logic and provide an
open surface to configure its execution.**

The `Controller` type is a type that is instantiated by the user to configure a
single unit of logic abstraction. The constructor less design allows the adding
of more knobs and switches in the future without generating churn. The controller
instances are then provided to the `Manager` that provides no configuration and
is created using an constructor. While the `Controller` has no public methods
the `Manager` provides public methods to control the execution of the logic.

The open controllers pattern may be combined with the classifiable models
pattern to allow customization of the abstracted logic's inner data structure.

### Framework Code

```go
package pkg

type Controller struct {
	Field string
	Model Model
}

func (c *Controller) execute() error {
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
	return nil
}
```

### Application Code

```go
package app

func Run() {
	manager := pkg.NewManager()

	manager.Add(&pkg.Controller{
		Field: "foo",
		Model: &Entity{},
	})

	manager.Run()
}
```

## Rich Contexts

**The rich contexts pattern is an alternative to Go's `context.Context` API
which tries to add more type safety instead of `interface{}` juggling.**

The framework provides a custom `Context` type that embeds a `context.Context`
for compatibility but only uses it to carry the externally provided context.
All framework related fields are openly accessible on the context. Ths context
is the passed to all called user handlers. The optional `Global` type and field
is a container that holds global state that can be inject at the beginning of
the processing chain.

### Framework Code

```go
package pkg

import "context"

type Global struct{
    Field string
}

type Context struct{
    context.Context
    
    Global     *Global
    Model      Model
    Controller Controller
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

func Example() *pkg.Controller {
    return &pkg.Controller{
        Handler1: func(ctx *pkg.Context) error {
            return nil
        },
        Handler2: makeHandler(),
        Handler3: wrapHandler(func(ctx *pkg.Context) error {
            return nil
        }),
    }
}
``` 
