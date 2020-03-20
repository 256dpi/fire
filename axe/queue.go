package axe

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"gopkg.in/tomb.v2"

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

	// BlockPeriod defines the duration after which a job may be returned again
	// from the board.
	//
	// It may take some time until the board is updated with the new state of
	// the job after a dequeue. The block period prevents another worker from
	// simultaneously trying to dequeue the job. If the initial worker failed to
	// dequeue the job it will be available again after the defined period.
	//
	// Default: 10s.
	BlockPeriod time.Duration

	store    *coal.Store
	reporter func(error)
	tasks    map[string]*Task
	boards   map[string]*board

	tomb tomb.Tomb
}

// NewQueue creates and returns a new queue.
func NewQueue(store *coal.Store, reporter func(error)) *Queue {
	return &Queue{
		store:    store,
		reporter: reporter,
		tasks:    make(map[string]*Task),
	}
}

// Add will add the specified task to the queue.
func (q *Queue) Add(task *Task) {
	// check existence
	if q.tasks[task.Name] != nil {
		panic(fmt.Sprintf(`axe: task with name "%s" already exists`, task.Name))
	}

	// save task
	q.tasks[task.Name] = task
}

// Enqueue will enqueue a job using the specified blueprint.
func (q *Queue) Enqueue(bp Blueprint) (*Job, error) {
	// enqueue job
	job, err := Enqueue(nil, q.store, bp)
	if err != nil {
		return nil, err
	}

	return job, nil
}

// Callback is a factory to create callbacks that can be used to enqueue jobs
// during request processing.
func (q *Queue) Callback(matcher fire.Matcher, cb func(ctx *fire.Context) Blueprint) *fire.Callback {
	return fire.C("axe/Queue.Callback", matcher, func(ctx *fire.Context) error {
		// get blueprint
		bp := cb(ctx)

		// check if controller uses same store
		if q.store == ctx.Controller.Store {
			// enqueue job using context store
			_, err := Enqueue(ctx, ctx.Store, bp)
			if err != nil {
				return err
			}
		} else {
			// enqueue job using queue store
			_, err := q.Enqueue(bp)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// Action is a factory to create an action that can be used to enqueue jobs.
func (q *Queue) Action(methods []string, cb func(ctx *fire.Context) Blueprint) *fire.Action {
	return fire.A("axe/Queue.Callback", methods, 0, func(ctx *fire.Context) error {
		// get blueprint
		bp := cb(ctx)

		// check if controller uses same store
		if q.store == ctx.Controller.Store {
			// enqueue job using context store
			_, err := Enqueue(ctx, ctx.Store, bp)
			if err != nil {
				return err
			}
		} else {
			// enqueue job using queue store
			_, err := q.Enqueue(bp)
			if err != nil {
				return err
			}
		}

		// respond with an empty object
		err := ctx.Respond(fire.Map{})
		if err != nil {
			return err
		}

		return nil
	})
}

// Run will start fetching jobs from the queue and process them.
func (q *Queue) Run() {
	// set default max lag
	if q.MaxLag == 0 {
		q.MaxLag = 100 * time.Millisecond
	}

	// set default block period
	if q.BlockPeriod == 0 {
		q.MaxLag = 10 * time.Second
	}

	// initialize boards
	q.boards = make(map[string]*board)

	// create boards
	for _, task := range q.tasks {
		q.boards[task.Name] = &board{
			jobs: make(map[coal.ID]*Job),
		}
	}

	// run process
	q.tomb.Go(q.process)
}

// Close will close the queue.
func (q *Queue) Close() {
	// kill and wait
	q.tomb.Kill(nil)
	_ = q.tomb.Wait()
}

func (q *Queue) process() error {
	// start tasks
	for _, task := range q.tasks {
		task.start(q)
	}

	// reconcile jobs
	stream := coal.Reconcile(q.store, &Job{}, func(model coal.Model) {
		q.update(model.(*Job))
	}, func(model coal.Model) {
		q.update(model.(*Job))
	}, nil, q.reporter)

	// await close
	<-q.tomb.Dying()

	// close stream
	stream.Close()

	return tomb.ErrDying
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

	// acquire board mutex
	board.Lock()
	defer board.Unlock()

	// get time
	now := time.Now()

	// return first available job
	for _, job := range board.jobs {
		if job.Available.Before(now) {
			// block job until the specified timeout has been reached
			job.Available = job.Available.Add(q.BlockPeriod)

			return job
		}
	}

	return nil
}
