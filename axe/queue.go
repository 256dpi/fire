package axe

import (
	"time"

	"github.com/256dpi/fire/coal"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

// Queue manages the queueing of jobs.
type Queue struct {
	Store *coal.Store
}

// Enqueue will enqueue a job using the specified name and data. If a delay
// is specified the job will not dequeued until the specified time has passed.
func (q *Queue) Enqueue(name string, data interface{}, delay time.Duration) (*Job, error) {
	// copy store
	store := q.Store.Copy()
	defer store.Close()

	// set default data
	if data == nil {
		data = bson.M{}
	}

	// prepare job
	job := coal.Init(&Job{
		Name:    name,
		Status:  StatusEnqueued,
		Created: time.Now(),
		Delayed: coal.T(time.Now().Add(delay)),
	}).(*Job)

	// marshall data
	raw, err := bson.Marshal(data)
	if err != nil {
		return nil, err
	}

	// marshall into job
	err = bson.Unmarshal(raw, &job.Data)
	if err != nil {
		return nil, err
	}

	// insert job
	err = store.C(job).Insert(job)
	if err != nil {
		return nil, err
	}

	return job, nil
}

// Dequeue will try to dequeue a job.
func (q *Queue) Dequeue(names []string, timeout time.Duration) (*Job, error) {
	// copy store
	store := q.Store.Copy()
	defer store.Close()

	// check names
	if len(names) == 0 {
		panic("at least one job name is required")
	}

	var job Job
	_, err := store.C(&Job{}).Find(bson.M{
		coal.F(&Job{}, "Name"): bson.M{
			"$in": names,
		},
		"$or": []bson.M{
			{
				coal.F(&Job{}, "Status"): bson.M{
					"$in": []Status{StatusEnqueued, StatusFailed},
				},
				coal.F(&Job{}, "Delayed"): bson.M{
					"$lte": time.Now(),
				},
			},
			{
				coal.F(&Job{}, "Status"): StatusDequeued,
				coal.F(&Job{}, "Started"): bson.M{
					"$lte": time.Now().Add(-timeout),
				},
			},
		},
	}).Sort("_id").Apply(mgo.Change{
		Update: bson.M{
			"$set": bson.M{
				coal.F(&Job{}, "Status"):  StatusDequeued,
				coal.F(&Job{}, "Started"): time.Now(),
			},
			"$inc": bson.M{
				coal.F(&Job{}, "Attempts"): 1,
			},
		},
		ReturnNew: true,
	}, &job)
	if err == mgo.ErrNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return &job, nil
}

// Fetch will load the job with the specified id.
func (q *Queue) Fetch(id bson.ObjectId) (*Job, error) {
	// copy store
	store := q.Store.Copy()
	defer store.Close()

	// find job
	var job Job
	err := store.C(&Job{}).FindId(id).One(&job)
	if err != nil {
		return nil, err
	}

	return &job, nil
}

// Complete will complete the specified job and set the specified result.
func (q *Queue) Complete(id bson.ObjectId, result bson.M) error {
	// copy store
	store := q.Store.Copy()
	defer store.Close()

	// update job
	err := store.C(&Job{}).UpdateId(id, bson.M{
		"$set": bson.M{
			coal.F(&Job{}, "Status"): StatusCompleted,
			coal.F(&Job{}, "Result"): result,
			coal.F(&Job{}, "Ended"):  time.Now(),
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// Fail will fail the specified job with the specified error. Delay can be set
// enforce a delay until the job can be dequeued again.
func (q *Queue) Fail(id bson.ObjectId, error string, delay time.Duration) error {
	// copy store
	store := q.Store.Copy()
	defer store.Close()

	// get time
	now := time.Now()

	// update job
	err := store.C(&Job{}).UpdateId(id, bson.M{
		"$set": bson.M{
			coal.F(&Job{}, "Status"):  StatusFailed,
			coal.F(&Job{}, "Error"):   error,
			coal.F(&Job{}, "Ended"):   now,
			coal.F(&Job{}, "Delayed"): now.Add(delay),
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// Cancel will cancel the specified job with the specified reason.
func (q *Queue) Cancel(id bson.ObjectId, reason string) error {
	// copy store
	store := q.Store.Copy()
	defer store.Close()

	// update job
	err := store.C(&Job{}).UpdateId(id, bson.M{
		"$set": bson.M{
			coal.F(&Job{}, "Status"): StatusCancelled,
			coal.F(&Job{}, "Reason"): reason,
			coal.F(&Job{}, "Ended"):  time.Now(),
		},
	})
	if err != nil {
		return err
	}

	return nil
}
