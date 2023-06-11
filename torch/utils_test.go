package torch

import (
	"testing"
	"time"

	"github.com/256dpi/xo"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/roast"
)

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-torch", xo.Crash)
var lungoStore = coal.MustOpen(nil, "test-fire-torch", xo.Crash)

var modelList = []coal.Model{&axe.Model{}, &testModel{}, &checkModel{}}

func withTester(t *testing.T, fn func(*testing.T, *coal.Store)) {
	t.Run("Mongo", func(t *testing.T) {
		coal.NewTester(mongoStore, modelList...).Clean()
		fn(t, mongoStore)
	})

	t.Run("Lungo", func(t *testing.T) {
		coal.NewTester(mongoStore, modelList...).Clean()
		fn(t, lungoStore)
	})
}

type operationTest struct {
	store     *coal.Store
	queue     *axe.Queue
	reactor   *Reactor
	operation *Operation
	tester    *roast.Tester
}

func testOperation(store *coal.Store, operation *Operation, fn func(env operationTest)) {
	queue := axe.NewQueue(axe.Options{
		Store:    store,
		Reporter: xo.Crash,
	})

	reactor := NewReactor(store, queue, operation)

	task := reactor.ScanTask()
	task.Periodicity = 0
	task.PeriodicJob = axe.Blueprint{}
	task.Interval = 10 * time.Millisecond
	queue.Add(task)

	task = reactor.ProcessTask()
	task.Interval = 10 * time.Millisecond
	queue.Add(task)

	queue.Run()
	defer queue.Close()

	group := fire.NewGroup(xo.Crash)

	group.Add(&fire.Controller{
		Store: store,
		Model: operation.Model,
		Modifiers: []*fire.Callback{
			reactor.Modifier(),
		},
	})

	tester := roast.NewTester(roast.Config{
		Store:   store,
		Models:  []coal.Model{operation.Model},
		Handler: group.Endpoint(""),
	})

	fn(operationTest{
		store:     store,
		queue:     queue,
		reactor:   reactor,
		operation: operation,
		tester:    tester,
	})
}
