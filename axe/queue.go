package axe

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"gopkg.in/tomb.v2"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

type board struct {
	sync.Mutex
	jobs map[coal.ID]*Model
}

// Blueprint describes a queueable job.
type Blueprint struct {
	// The job to be enqueued.
	Job Job

	// The job delay. If specified, the job will not be dequeued until the
	// specified duration has passed.
	Delay time.Duration

	// The job isolation. If specified, the job will only be enqueued if no job
	// has been executed in the specified duration.
	Isolation time.Duration
}

// Options defines queue options.
type Options struct {
	// The store used to manage jobs.
	Store *coal.Store

	// The maximum amount of lag that should be applied to every dequeue attempt.
	//
	// By default, multiple workers compete with each other when getting jobs
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

// Queue manages job queueing.
type Queue struct {
	options Options
	tasks   map[string]*Task
	boards  map[string]*board
	tomb    tomb.Tomb
}

// NewQueue creates and returns a new queue.
func NewQueue(options Options) *Queue {
	// set default max lag
	if options.MaxLag == 0 {
		options.MaxLag = 100 * time.Millisecond
	}

	// set default block period
	if options.BlockPeriod == 0 {
		options.BlockPeriod = 10 * time.Second
	}

	return &Queue{
		options: options,
		tasks:   make(map[string]*Task),
	}
}

// Add will add the specified task to the queue.
func (q *Queue) Add(task *Task) {
	// safety check
	if q.boards != nil {
		panic("axe: unable to add task to running queue")
	}

	// prepare task
	task.prepare()

	// get name
	name := GetMeta(task.Job).Name

	// check existence
	if q.tasks[name] != nil {
		panic(fmt.Sprintf(`axe: task with name "%s" already exists`, name))
	}

	// save task
	q.tasks[name] = task
}

// Enqueue will enqueue a job. If the context carries a transaction it must be
// associated with the store that is also used by the queue.
func (q *Queue) Enqueue(ctx context.Context, job Job, delay, isolation time.Duration) (bool, error) {
	return Enqueue(ctx, q.options.Store, job, delay, isolation)
}

// Callback is a factory to create callbacks that can be used to enqueue jobs
// during request processing.
func (q *Queue) Callback(matcher fire.Matcher, cb func(ctx *fire.Context) Blueprint) *fire.Callback {
	return fire.C("axe/Queue.Callback", 0, matcher, func(ctx *fire.Context) error {
		// get blueprint
		bp := cb(ctx)

		// check transaction
		ok, ts := coal.GetTransaction(ctx)

		// check if transaction store is different
		if ok && ts != q.options.Store {
			// enqueue job outside of transaction
			_, err := q.Enqueue(nil, bp.Job, bp.Delay, bp.Isolation)
			if err != nil {
				return err
			}
		} else {
			// otherwise enqueue with potential transaction
			_, err := q.Enqueue(ctx, bp.Job, bp.Delay, bp.Isolation)
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

		// check transaction
		ok, ts := coal.GetTransaction(ctx)

		// check if transaction store is different
		if ok && ts != q.options.Store {
			// enqueue job outside of transaction
			_, err := q.Enqueue(nil, bp.Job, bp.Delay, bp.Isolation)
			if err != nil {
				return err
			}
		} else {
			// otherwise enqueue with potential transaction
			_, err := q.Enqueue(ctx, bp.Job, bp.Delay, bp.Isolation)
			if err != nil {
				return err
			}
		}

		// respond with an empty object
		err := ctx.Respond(stick.Map{})
		if err != nil {
			return err
		}

		return nil
	})
}

// Run will start fetching jobs from the queue and execute them. It will return
// a channel that is closed once the queue has been synced and is available.
func (q *Queue) Run() chan struct{} {
	// initialize boards
	q.boards = make(map[string]*board)

	// create boards
	for _, task := range q.tasks {
		name := GetMeta(task.Job).Name
		q.boards[name] = &board{
			jobs: make(map[coal.ID]*Model),
		}
	}

	// prepare channel
	synced := make(chan struct{})

	// run process
	q.tomb.Go(func() error {
		return q.process(synced)
	})

	return synced
}

// Close will close the queue.
func (q *Queue) Close() {
	// kill and wait
	q.tomb.Kill(nil)
	_ = q.tomb.Wait()
}

func (q *Queue) process(synced chan struct{}) error {
	// start tasks
	for _, task := range q.tasks {
		task.start(q)
	}

	// reconcile jobs
	var once sync.Once
	stream := coal.Reconcile(q.options.Store, &Model{}, func() {
		once.Do(func() {
			close(synced)
		})
	}, func(model coal.Model) {
		q.update(model.(*Model))
	}, func(model coal.Model) {
		q.update(model.(*Model))
	}, nil, q.options.Reporter)

	// await close
	<-q.tomb.Dying()

	// close stream
	stream.Close()

	return tomb.ErrDying
}

func (q *Queue) update(job *Model) {
	// get board
	board, ok := q.boards[job.Name]
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
		lag := time.Duration(rand.Int63n(int64(q.options.MaxLag)))
		job.Available = job.Available.Add(lag)

		// update job
		board.jobs[job.ID()] = job
	case Completed, Cancelled:
		// remove job
		delete(board.jobs, job.ID())
	}
}

func (q *Queue) get(name string) (coal.ID, bool) {
	// get board
	board := q.boards[name]

	// lock board
	board.Lock()
	defer board.Unlock()

	// get time
	now := time.Now()

	// return first available job
	for _, job := range board.jobs {
		if job.Available.Before(now) {
			// block job until the specified timeout has been reached
			job.Available = job.Available.Add(q.options.BlockPeriod)

			return job.ID(), true
		}
	}

	return coal.ID{}, false
}
