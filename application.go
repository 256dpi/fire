package fire

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
	"gopkg.in/mgo.v2"
)

// An Application provides an out-of-the-box configuration of components to
// get started with building JSON APIs.
type Application struct {
	set    *Set
	router *echo.Echo
}

// New creates and returns a new Application.
func New(mongoURI, prefix string) *Application {
	// create router
	router := echo.New()

	// connect to database
	sess, err := mgo.Dial(mongoURI)
	if err != nil {
		panic(err)
	}

	// get db
	db := sess.DB("")

	set := NewSet(db, router, prefix)

	return &Application{
		set:    set,
		router: router,
	}
}

// Mount will add controllers to the set and register them on the router.
//
// Note: Each controller should only be mounted once.
func (a *Application) Mount(controllers ...*Controller) {
	a.set.Mount(controllers...)
}

// Router will return the internally used echo instance.
func (a *Application) Router() *echo.Echo {
	return a.router
}

// Run will run the application using the passed server.
func (a *Application) Run(server engine.Server) {
	a.router.Run(server)
}

// Start will run the application on the specified address.
func (a *Application) Start(addr string) {
	a.Run(standard.New(addr))
}
