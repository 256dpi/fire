package axe

import (
	"math/rand"
	"sync"
	"time"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

type board struct {
	sync.Mutex
	jobs map[coal.ID]*Job
}

// Queue manages the queueing of jobs.
type Queue struct {
	// MaxLag defines the maximum amount of lag that should be applied to every
	// dequeue attempt.
	//
	// By default multiple processes compete with each other when getting jobs
	// from the same queue. An artificial lag prevents multiple simultaneous
	// dequeue attempts and allows the worker with the smallest lag to dequeue
	// the job and inform the other processes to prevent another dequeue attempt.
	//
	// Default: 100ms.
	MaxLag time.Duration

	// DequeueInterval defines the time after a worker might try to dequeue a job
	// again that has not yet been associated.
	//
	// Default: 2s.
	DequeueInterval time.Duration

	store  *coal.Store
	tasks  []string
	boards map[string]*board
}

// NewQueue creates and returns a new queue.
func NewQueue(store *coal.Store) *Queue {
	return &Queue{
		store: store,
	}
}

// Enqueue will enqueue a job using the specified name and data. If a delay
// is specified the job will not dequeued until the specified time has passed.
func (q *Queue) Enqueue(name string, model Model, delay time.Duration) (*Job, error) {
	// enqueue job
	job, err := Enqueue(q.store, nil, name, model, delay)
	if err != nil {
		return nil, err
	}

	return job, nil
}

// Callback is a factory to create callbacks that can be used to enqueue jobs
// during request processing.
func (q *Queue) Callback(name string, delay time.Duration, matcher fire.Matcher, cb func(ctx *fire.Context) Model) *fire.Callback {
	return fire.C("axe/Queue.Callback", matcher, func(ctx *fire.Context) error {
		// set task tag
		ctx.Tracer.Tag("task", name)

		// get model
		var model Model
		if cb != nil {
			model = cb(ctx)
		}

		// check if controller uses same store
		if q.store == ctx.Controller.Store {
			// enqueue job using context store
			_, err := Enqueue(ctx.Store, ctx.Session, name, model, delay)
			if err != nil {
				return err
			}
		} else {
			// enqueue job using queue store
			_, err := q.Enqueue(name, model, delay)
			if err != nil {
				return err
			}
		}

		// respond with an empty object
		if ctx.Operation.Action() {
			err := ctx.Respond(fire.Map{})
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (q *Queue) start(p *Pool) {
	// set default max lag
	if q.MaxLag == 0 {
		q.MaxLag = 100 * time.Millisecond
	}

	// set default dequeue interval
	if q.DequeueInterval == 0 {
		q.DequeueInterval = 2 * time.Second
	}

	// initialize boards
	q.boards = make(map[string]*board)

	// create boards
	for _, task := range q.tasks {
		q.boards[task] = &board{
			jobs: make(map[coal.ID]*Job),
		}
	}

	// run watcher
	go q.watcher(p)
}

func (q *Queue) watcher(p *Pool) {
	// reconcile jobs
	stream := coal.Reconcile(q.store, &Job{}, func(model coal.Model) {
		q.update(model.(*Job))
	}, func(model coal.Model) {
		q.update(model.(*Job))
	}, nil, p.reporter)

	// await close
	<-p.closed

	// close stream
	stream.Close()
}

func (q *Queue) update(job *Job) {
	// get board
	board, ok := q.boards[job.Name]
	if !ok {
		return
	}

	// lock board
	board.Lock()
	defer board.Unlock()

	// handle job
	switch job.Status {
	case StatusEnqueued, StatusDequeued, StatusFailed:
		// apply random lag if configured
		if q.MaxLag > 0 {
			lag := time.Duration(rand.Int63n(int64(q.MaxLag)))
			job.Available = job.Available.Add(lag)
		}

		// updated job
		board.jobs[job.ID()] = job
	case StatusCompleted, StatusCancelled:
		// remove job
		delete(board.jobs, job.ID())
	}
}

func (q *Queue) get(name string) *Job {
	// get board
	board := q.boards[name]

	// acquire mutex
	board.Lock()
	defer board.Unlock()

	// get time
	now := time.Now()

	// get a random job
	var job *Job
	for _, job = range board.jobs {
		// ignore jobs that are not yet available
		if job.Available.After(now) {
			job = nil
			continue
		}

		break
	}

	// check job
	if job == nil {
		return nil
	}

	// make job unavailable until the specified timeout has been reached. this
	// ensures that a job dequeued by a crashed worker will be picked up by
	// another worker at some point
	job.Available = job.Available.Add(q.DequeueInterval)

	return job
}
