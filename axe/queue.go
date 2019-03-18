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
	Store *coal.Store

	tasks  []string
	boards map[string]*board
}

func (q *Queue) init() {
	// initialize boards
	q.boards = make(map[string]*board)

	// create boards
	for _, task := range q.tasks {
		q.boards[task] = &board{
			jobs: make(map[bson.ObjectId]*Job),
		}
	}
}

func (q *Queue) run(p *Pool) {
	// create stream
	s := coal.NewStream(q.Store, &Job{})
	s.Reporter = p.Reporter

	// prepare channel
	open := make(chan struct{})

	// open stream
	s.Tail(func(e coal.Event, id bson.ObjectId, m coal.Model) {
		// ignore deleted events
		if e == coal.Deleted {
			return
		}

		// get job
		job := m.(*Job)

		// get board
		board := q.boards[job.Name]

		// handle job
		switch job.Status {
		case StatusEnqueued, StatusFailed:
			// add job
			board.Lock()
			board.jobs[job.ID()] = job
			board.Unlock()
		case StatusDequeued, StatusCompleted, StatusCancelled:
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

	// TODO: Add existing jobs.

	// TODO: Resync every minute and add missing jobs?

	select {
	case <-p.closed:
		// close stream
		s.Close()

		return
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
		// ignore jobs that are delayed
		if job.Delayed.After(now) {
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
