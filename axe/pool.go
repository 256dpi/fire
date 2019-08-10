package axe

import (
	"fmt"
)

// Pool manages tasks and queues.
type Pool struct {
	reporter func(error)
	tasks    map[string]*Task
	queues   map[*Queue]bool
	closed   chan struct{}
}

// NewPool creates and returns a new pool.
func NewPool(reporter func(error)) *Pool {
	return &Pool{
		reporter: reporter,
		tasks:    make(map[string]*Task),
		queues:   make(map[*Queue]bool),
		closed:   make(chan struct{}),
	}
}

// Add will add the specified task and its queue to the pool.
func (p *Pool) Add(task *Task) {
	// check existence
	if p.tasks[task.Name] != nil {
		panic(fmt.Sprintf(`axe: task with name "%s" already exists`, task.Name))
	}

	// save task
	p.tasks[task.Name] = task

	// add task to queue
	task.Queue.tasks = append(task.Queue.tasks, task.Name)

	// save queue
	p.queues[task.Queue] = true
}

// Run will launch the queue watchers and task workers in the background.
func (p *Pool) Run() {
	// start all queues
	for queue := range p.queues {
		queue.start(p)
	}

	// start all tasks
	for _, task := range p.tasks {
		task.start(p)
	}
}

// Close will close the pool.
func (p *Pool) Close() {
	close(p.closed)
}
