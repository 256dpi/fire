package axe

import (
	"fmt"
)

type Pool struct {
	tasks  map[string]*Task
	queues map[*Queue]bool

	closed chan struct{}

	Reporter func(error)
}

func NewPool() *Pool {
	return &Pool{
		tasks:  make(map[string]*Task),
		queues: make(map[*Queue]bool),
		closed: make(chan struct{}),
	}
}

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

func (p *Pool) Run() {
	// init all queues
	for queue := range p.queues {
		queue.init()
	}

	// run all queues
	for queue := range p.queues {
		go queue.run(p)
	}

	// run all tasks
	for _, task := range p.tasks {
		go task.run(p)
	}
}

// Close will closer the pool.
func (p *Pool) Close() {
	// close closer
	close(p.closed)
}
