package axe

import (
	"sync"
	"time"

	"github.com/256dpi/fire/coal"
	"github.com/globalsign/mgo/bson"
)

type board struct {
	sync.Mutex
	jobs map[bson.ObjectId]*Job
}

// Queue manages the queueing of jobs.
type Queue struct {
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
	// create stream
	s := coal.NewStream(q.store, &Job{})
	s.Reporter = p.Reporter

	// prepare channel
	open := make(chan struct{})

	// open stream
	s.Open(func(e coal.Event, id bson.ObjectId, m coal.Model) {
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
		close(open)
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
