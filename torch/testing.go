package torch

import (
	"time"

	"github.com/256dpi/xo"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/roast"
)

// Env is a testing environment.
type Env struct {
	*roast.Tester
	Queue     *axe.Queue
	Reactor   *Reactor
	Operation *Operation
}

// Test will create and yield a testing environment for the specified operation.
func Test(store *coal.Store, operation *Operation, fn func(env Env)) {
	// ensure store
	if store == nil {
		store = coal.MustOpen(nil, "test", xo.Crash)
	}

	// create queue
	queue := axe.NewQueue(axe.Options{
		Store:    store,
		Reporter: xo.Crash,
	})

	// create reactor
	reactor := NewReactor(store, queue, xo.Crash, operation)

	// add scan task
	task := reactor.ScanTask()
	task.Periodicity = 0
	task.PeriodicJob = axe.Blueprint{}
	task.Interval = 10 * time.Millisecond
	queue.Add(task)

	// add process task
	task = reactor.ProcessTask()
	task.Interval = 10 * time.Millisecond
	queue.Add(task)

	// run queue
	queue.Run()
	defer queue.Close()

	// create group
	group := fire.NewGroup(xo.Crash)

	// add controller
	group.Add(&fire.Controller{
		Store: store,
		Model: operation.Model,
		Modifiers: []*fire.Callback{
			reactor.Modifier(),
		},
	})

	// create tester
	tester := roast.NewTester(roast.Config{
		Store:   store,
		Models:  []coal.Model{operation.Model},
		Handler: group.Endpoint(""),
	})

	// yield env
	fn(Env{
		Tester:    tester,
		Queue:     queue,
		Reactor:   reactor,
		Operation: operation,
	})
}

// Scan will queue and await a scan for the tested operation. It will return
// the number of queued and executed process jobs.
func (e *Env) Scan() (int, error) {
	n, err := axe.AwaitJob(e.Store, 0, NewScanJob(e.Operation.Name))
	if err != nil {
		return 0, err
	}
	return n - 1, nil
}

// Process will enqueue and await a process job for the tested operation and
// specified model.
func (e *Env) Process(model coal.Model) error {
	_, err := axe.AwaitJob(e.Store, 0, NewProcessJob(e.Operation.Name, model.ID()))
	return err
}
