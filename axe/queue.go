package axe

import (
	"math/rand"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

type board struct {
	sync.Mutex
	jobs map[primitive.ObjectID]*Job
}

// Queue manages the queueing of jobs.
type Queue struct {
	// MaxLag defines the maximum amount of lag that should be applied to every
	// dequeue attempt.
	//
	// By default multiple processes compete with each other when getting jobs
	// from the same queue. An artificial lag prevents multiple simultaneous
	// dequeue attempts and allows the worker with the smallest lag to dequeue
	// the job and inform the other processed to prevent another dequeue attempt.
	//
	// Default: 0.
	MaxLag time.Duration

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
	// initialize boards
	q.boards = make(map[string]*board)

	// create boards
	for _, task := range q.tasks {
		q.boards[task] = &board{
			jobs: make(map[primitive.ObjectID]*Job),
		}
	}

	// run watcher
	go q.watcher(p)
}

func (q *Queue) watcher(p *Pool) {
	// open stream
	s := coal.OpenStream(q.store, &Job{}, nil, func(e coal.Event, id primitive.ObjectID, m coal.Model, token []byte) error {
		// check for opened event
		if e == coal.Opened {
			return q.fill()
		}

		// ignore resumed events
		if e == coal.Resumed {
			return nil
		}

		// ignore deleted events
		if e == coal.Deleted {
			return nil
		}

		// get job
		job := m.(*Job)

		// get board
		board, ok := q.boards[job.Name]
		if !ok {
			return nil
		}

		// handle job
		switch job.Status {
		case StatusEnqueued, StatusDequeued, StatusFailed:
			// apply random lag if configured
			if q.MaxLag > 0 {
				lag := time.Duration(rand.Int63n(int64(q.MaxLag)))
				job.Available = job.Available.Add(lag)
			}

			// add job
			board.Lock()
			board.jobs[job.ID()] = job
			board.Unlock()
		case StatusCompleted, StatusCancelled:
			// remove job
			board.Lock()
			delete(board.jobs, job.ID())
			board.Unlock()
		}

		return nil
	}, func(err error) bool {
		// report error
		p.Reporter(err)

		return true
	})

	// wait close
	select {
	case <-p.closed:
		// close stream
		s.Close()

		return
	}
}

func (q *Queue) fill() error {
	// get existing jobs
	var jobs []*Job
	cursor, err := q.store.C(&Job{}).Find(nil, bson.M{
		coal.F(&Job{}, "Status"): bson.M{
			"$in": []Status{StatusEnqueued, StatusDequeued, StatusFailed},
		},
	})
	if err != nil {
		return err
	}

	// decode all results
	err = cursor.All(nil, &jobs)
	if err != nil {
		return err
	}

	// close cursor
	err = cursor.Close(nil)
	if err != nil {
		return err
	}

	// add jobs
	for _, job := range jobs {
		// get board
		board, ok := q.boards[job.Name]
		if !ok {
			continue
		}

		// add job
		board.Lock()
		board.jobs[job.ID()] = job
		board.Unlock()
	}

	return nil
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
		// ignore jobs that are delayed
		if job.Available.After(now) {
			job = nil
			continue
		}

		break
	}

	// return nil if not found
	if job == nil {
		return nil
	}

	// delete job
	delete(board.jobs, job.ID())

	return job
}
