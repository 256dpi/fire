package axe

import (
	"math/rand"
	"sync"
	"time"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"github.com/globalsign/mgo/bson"
)

type board struct {
	sync.Mutex
	jobs map[bson.ObjectId]*Job
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
func (q *Queue) Enqueue(name string, data Model, delay time.Duration) (*Job, error) {
	// copy store
	store := q.store.Copy()
	defer store.Close()

	// enqueue job
	job, err := Enqueue(store, name, data, delay)
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

		// get data
		var data Model
		if cb != nil {
			data = cb(ctx)
		}

		// check if controller uses same store
		if q.store == ctx.Controller.Store {
			// enqueue job using context store
			_, err := Enqueue(ctx.Store, name, data, delay)
			if err != nil {
				return err
			}
		} else {
			// enqueue job using queue store
			_, err := q.Enqueue(name, data, delay)
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
			jobs: make(map[bson.ObjectId]*Job),
		}
	}

	// run watcher
	go q.watcher(p)
}

func (q *Queue) watcher(p *Pool) {
	// prepare channel
	open := make(chan struct{})

	// open stream
	s := coal.OpenStream(q.store, &Job{}, nil, func(e coal.Event, id bson.ObjectId, m coal.Model, token []byte) {
		// ignore deleted events
		if e == coal.Deleted {
			return
		}

		// get job
		job := m.(*Job)

		// get board
		board, ok := q.boards[job.Name]
		if !ok {
			return
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
	}, func() {
		// signal open
		close(open)
	}, func(err error) bool {
		// report error
		p.Reporter(err)

		return true
	})

	// await steam open
	select {
	case <-open:
		// continue
	case <-p.closed:
		// close stream
		s.Close()

		return
	}

	// fill queue with existing jobs
	err := q.fill()
	if err != nil {
		if p.Reporter != nil {
			p.Reporter(err)
		}
	}

	// wait close
	select {
	case <-p.closed:
		// close stream
		s.Close()

		return
	}
}

func (q *Queue) fill() error {
	// copy store
	store := q.store.Copy()
	defer store.Close()

	// get existing jobs
	var jobs []*Job
	err := store.C(&Job{}).Find(bson.M{
		coal.F(&Job{}, "Status"): bson.M{
			"$in": []Status{StatusEnqueued, StatusDequeued, StatusFailed},
		},
	}).All(&jobs)
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
