package axe

import (
	"github.com/256dpi/xo"

	"github.com/256dpi/fire/coal"
)

// TODO: Use board data available through Queue?

// AwaitJob will enqueue the specified job and wait until it and all other
// jobs queued during its execution are finished. It will return the number
// of processed jobs.
func AwaitJob(store *coal.Store, job Job) (int, error) {
	return Await(store, func() error {
		// enqueue job
		ok, err := Enqueue(nil, store, job, 0, 0)
		if err != nil {
			return err
		} else if !ok {
			return xo.F("enqueue failed")
		}

		return nil
	})
}

// Await will await all jobs created during the execution of the callback. It
// will wait for at least one job to complete and return the number of
// processed jobs. If a job fails or is cancelled its reasons is returned as
// an error.
func Await(store *coal.Store, fns ...func() error) (int, error) {
	// prepare state
	var num int
	jobs := map[coal.ID]struct{}{}
	done := make(chan error, 1)

	// open stream
	stream := coal.OpenStream(store, &Model{}, nil, func(event coal.Event, id coal.ID, model coal.Model, err error, token []byte) error {
		// run callbacks on open
		if event == coal.Opened && len(fns) > 0 {
			for _, fn := range fns {
				err := fn()
				if err != nil {
					return err
				}
			}

			return nil
		}

		// handle job creation
		if event == coal.Created {
			num++
			jobs[id] = struct{}{}
			return nil
		}

		// handle job updates
		if event == coal.Updated {
			job := model.(*Model)
			switch job.State {
			case Dequeued:
				if _, ok := jobs[id]; !ok {
					num++
					jobs[id] = struct{}{}
				}
				return nil
			case Completed:
				delete(jobs, id)
				if len(jobs) == 0 {
					close(done)
					return coal.ErrStop.Wrap()
				}
				return nil
			case Failed, Cancelled:
				reason := job.Events[len(job.Events)-1].Reason
				if reason == "" {
					reason = job.Status
				}
				done <- xo.F("failed: %s", reason)
				close(done)
				return coal.ErrStop.Wrap()
			}
		}

		// handle errors
		if event == coal.Errored {
			xo.Panic(err)
		}

		return nil
	})

	// await done
	err := <-done

	// close stream
	stream.Close()

	return num, err
}
