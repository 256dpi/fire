package kiln

import (
	"fmt"
	"sync"
	"time"

	"gopkg.in/tomb.v2"

	"github.com/256dpi/fire/coal"
)

type board struct {
	sync.Mutex
	processes map[coal.ID]*Model
}

// Options defines coordinator options.
type Options struct {
	// The store used to manage jobs.
	Store *coal.Store

	// The maximum amount of lag that should be applied to every dequeue attempt.
	//
	// By default multiple workers compete with each other when getting jobs
	// from the same queue. An artificial lag limits multiple simultaneous
	// dequeue attempts and allows the worker with the smallest lag to dequeue
	// the job and inform the other workers to limit parallel dequeue attempts.
	//
	// Default: 100ms.
	MaxLag time.Duration

	// The duration after which a job may be returned again from the board.
	//
	// It may take some time until the board is updated with the new state of
	// the job after a dequeue. The block period prevents another worker from
	// simultaneously trying to dequeue the job. If the initial worker failed to
	// dequeue the job it will be available again after the defined period.
	//
	// Default: 10s.
	BlockPeriod time.Duration

	// The callback that is called with job errors.
	Reporter func(error)
}

type Scheduler struct {
	options   Options
	executors map[string]*Executor
	boards    map[string]*board
	tomb      tomb.Tomb
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		executors: map[string]*Executor{},
	}
}

func (s *Scheduler) Add(exec *Executor) {
	// safety check
	if s.boards != nil {
		panic("kiln: unable to add executor to running scheduler")
	}

	// prepare executor
	exec.prepare()

	// get name
	name := GetMeta(exec.Process).Name

	// check existence
	if s.executors[name] != nil {
		panic(fmt.Sprintf(`kiln: executor with name "%s" already exists`, name))
	}

	// save executor
	s.executors[name] = exec
}

// TODO: Schedule, Callback, Action...

func (s *Scheduler) Run() chan struct{} {
	// initialize boards
	s.boards = make(map[string]*board)

	// create boards
	for _, exec := range s.executors {
		name := GetMeta(exec.Process).Name
		s.boards[name] = &board{
			processes: make(map[coal.ID]*Model),
		}
	}

	// prepare channel
	synced := make(chan struct{})

	// run process
	s.tomb.Go(func() error {
		return s.process(synced)
	})

	return synced
}

// Close will close the queue.
func (s *Scheduler) Close() {
	// kill and wait
	s.tomb.Kill(nil)
	_ = s.tomb.Wait()
}

func (s *Scheduler) process(synced chan struct{}) error {
	// start tasks
	// for _, exec := range s.executors {
	// 	exec.start(s)
	// }

	// reconcile processes
	var once sync.Once
	stream := coal.Reconcile(s.options.Store, &Model{}, func() {
		once.Do(func() {
			close(synced)
		})
	}, func(model coal.Model) {
		s.update(model.(*Model))
	}, func(model coal.Model) {
		s.update(model.(*Model))
	}, nil, s.options.Reporter)

	// await close
	<-s.tomb.Dying()

	// close stream
	stream.Close()

	return tomb.ErrDying
}

func (s *Scheduler) update(job *Model) {
	// get board
	board, ok := s.boards[job.Name]
	if !ok {
		return
	}

	// lock board
	board.Lock()
	defer board.Unlock()

	// handle job
	switch job.State {
	case Enqueued, Dequeued, Failed:
		// apply random lag
		lag := time.Duration(rand.Int63n(int64(s.options.MaxLag)))
		job.Available = job.Available.Add(lag)

		// update job
		board.jobs[job.ID()] = job
	case Completed, Cancelled:
		// remove job
		delete(board.jobs, job.ID())
	}
}

func (s *Scheduler) get(name string) (coal.ID, bool) {
	// get board
	board := s.boards[name]

	// lock board
	board.Lock()
	defer board.Unlock()

	// get time
	now := time.Now()

	// return first available job
	for _, job := range board.jobs {
		if job.Available.Before(now) {
			// block job until the specified timeout has been reached
			job.Available = job.Available.Add(s.options.BlockPeriod)

			return job.ID(), true
		}
	}

	return coal.ID{}, false
}
