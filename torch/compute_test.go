package torch

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

type computeModel struct {
	coal.Base `json:"-" bson:",inline" coal:"compute"`
	Input     string
	Status    *Status
	Output    string
	stick.NoValidation
}

func TestComputeScan(t *testing.T) {
	testOperation(t, Compute(Computation{
		Name:   "Status",
		Model:  &computeModel{},
		Hasher: StringHasher("Input"),
		Computer: StringComputer("Input", "Output", func(ctx *Context, input string) (string, error) {
			return strings.ToUpper(input), nil
		}),
		Releaser: func(ctx *Context) error {
			ctx.Change("$set", "Output", "")
			return nil
		},
	}), func(env operationTest) {
		model := env.tester.Insert(&computeModel{
			Base: coal.B(),
		}).(*computeModel)

		/* missing input */

		n, err := axe.AwaitJob(env.store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 3, n)

		env.tester.Refresh(model)
		assert.Zero(t, model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     "",
			Valid:    true,
		}, model.Status)
		assert.NotZero(t, model.Status.Updated)

		/* first input */

		updated := model.Status.Updated

		model.Input = "Hello world!"
		model.Status.Valid = false
		env.tester.Replace(model)

		n, err = axe.AwaitJob(env.tester.Store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 3, n)

		env.tester.Refresh(model)
		assert.Equal(t, "HELLO WORLD!", model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     Hash("Hello world!"),
			Valid:    true,
		}, model.Status)
		assert.True(t, model.Status.Updated.After(updated))

		/* same input */

		oldOutput := model.Output
		oldStatus := model.Status

		n, err = axe.AwaitJob(env.tester.Store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 2, n)

		env.tester.Refresh(model)
		assert.NotNil(t, model.Output)
		assert.Equal(t, oldOutput, model.Output)
		assert.Equal(t, oldStatus, model.Status)

		/* new input */

		updated = model.Status.Updated

		model.Input = "What's up?"
		env.tester.Replace(model)

		n, err = axe.AwaitJob(env.tester.Store, 0, NewProcessJob("torch/Compute/Status", model.ID()))
		assert.NoError(t, err)
		assert.Equal(t, 1, n)

		env.tester.Refresh(model)
		assert.Equal(t, "WHAT'S UP?", model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     Hash("What's up?"),
			Valid:    true,
		}, model.Status)
		assert.True(t, model.Status.Updated.After(updated))

		/* leftover input */

		updated = model.Status.Updated

		model.Input = ""
		env.tester.Replace(model)

		n, err = axe.AwaitJob(env.tester.Store, 0, NewProcessJob("torch/Compute/Status", model.ID()))
		assert.NoError(t, err)
		assert.Equal(t, 1, n)

		env.tester.Refresh(model)
		assert.Zero(t, model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     "",
			Valid:    true,
		}, model.Status)
		assert.True(t, model.Status.Updated.After(updated))
	})
}

func TestComputeProcess(t *testing.T) {
	testOperation(t, Compute(Computation{
		Name:   "Status",
		Model:  &computeModel{},
		Hasher: StringHasher("Input"),
		Computer: StringComputer("Input", "Output", func(ctx *Context, input string) (string, error) {
			return strings.ToUpper(input), nil
		}),
		Releaser: func(ctx *Context) error {
			ctx.Change("$set", "Output", "")
			return nil
		},
	}), func(env operationTest) {
		var model *computeModel

		/* missing input */

		env.tester.Await(t, 50*time.Millisecond, func() {
			model = env.tester.Create(t, &computeModel{}, nil, nil).Model.(*computeModel)
		})

		env.tester.Refresh(model)
		assert.Zero(t, model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     "",
			Valid:    true,
		}, model.Status)

		/* first input */

		updated := model.Status.Updated

		model.Input = "Hello world!"
		env.tester.Await(t, 0, func() {
			model = env.tester.Update(t, model, nil, nil).Model.(*computeModel)
			assert.Zero(t, model.Output)
			assert.Equal(t, &Status{
				Progress: 0,
				Updated:  model.Status.Updated,
				Hash:     "",
				Valid:    false,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(updated))
		})

		updated = model.Status.Updated

		env.tester.Refresh(model)
		assert.Equal(t, "HELLO WORLD!", model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     Hash("Hello world!"),
			Valid:    true,
		}, model.Status)
		assert.True(t, model.Status.Updated.After(updated))

		/* same input */

		before := model.Output

		env.tester.Await(t, 50*time.Millisecond, func() {
			env.tester.Update(t, model, nil, nil)
		})

		env.tester.Refresh(model)
		assert.Equal(t, before, model.Output)

		/* new input */

		updated = model.Status.Updated

		model.Input = "What's up?"
		env.tester.Await(t, 0, func() {
			model = env.tester.Update(t, model, nil, nil).Model.(*computeModel)
			assert.Zero(t, model.Output)
			assert.Equal(t, &Status{
				Progress: 0,
				Updated:  model.Status.Updated,
				Hash:     "",
				Valid:    false,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(updated))
		})

		updated = model.Status.Updated

		env.tester.Refresh(model)
		assert.Equal(t, "WHAT'S UP?", model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     Hash("What's up?"),
			Valid:    true,
		}, model.Status)
		assert.True(t, model.Status.Updated.After(updated))

		/* leftover link */

		updated = model.Status.Updated

		model.Input = ""
		env.tester.Await(t, 50*time.Millisecond, func() {
			env.tester.Update(t, model, nil, nil)
		})

		env.tester.Refresh(model)
		assert.Zero(t, model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     "",
			Valid:    true,
		}, model.Status)
		assert.True(t, model.Status.Updated.After(updated))
	})
}

func TestComputeProgress(t *testing.T) {
	testOperation(t, Compute(Computation{
		Name:   "Status",
		Model:  &computeModel{},
		Hasher: StringHasher("Input"),
		Computer: func(ctx *Context) error {
			for i := 0; i < 4; i++ {
				time.Sleep(50 * time.Millisecond)
				err := ctx.Progress(float64(i) * 0.25)
				if err != nil {
					return err
				}
			}
			m := ctx.Model.(*computeModel)
			ctx.Change("$set", "Output", strings.ToUpper(m.Input))
			return nil
		},
		Releaser: func(ctx *Context) error {
			ctx.Change("$set", "Output", "")
			return nil
		},
	}), func(env operationTest) {
		var progress []float64
		stream := coal.Reconcile(env.store, &computeModel{}, nil, nil, func(model coal.Model) {
			progress = append(progress, model.(*computeModel).Status.Progress)
		}, nil, nil)
		defer stream.Close()

		var model *computeModel
		env.tester.Await(t, 0, func() {
			model = env.tester.Create(t, &computeModel{
				Input: "Hello world!",
			}, nil, nil).Model.(*computeModel)
		})

		env.tester.Refresh(model)
		assert.Equal(t, "HELLO WORLD!", model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     Hash("Hello world!"),
			Valid:    true,
		}, model.Status)
		assert.Equal(t, []float64{0, 0.25, 0.5, 0.75, 1}, progress)
	})
}

func TestComputeKeepOutdated(t *testing.T) {
	testOperation(t, Compute(Computation{
		Name:   "Status",
		Model:  &computeModel{},
		Hasher: StringHasher("Input"),
		Computer: StringComputer("Input", "Output", func(ctx *Context, input string) (string, error) {
			return strings.ToUpper(input), nil
		}),
		Releaser: func(ctx *Context) error {
			ctx.Change("$set", "Output", "")
			return nil
		},
		KeepOutdated: true,
	}), func(env operationTest) {
		var model *computeModel

		/* first input */

		env.tester.Await(t, 0, func() {
			model = env.tester.Create(t, &computeModel{
				Input: "Hello world!",
			}, nil, nil).Model.(*computeModel)
		})

		env.tester.Refresh(model)
		assert.Equal(t, "HELLO WORLD!", model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     Hash("Hello world!"),
			Valid:    true,
		}, model.Status)

		/* new input */

		model.Input = "What's up?"
		env.tester.Await(t, 0, func() {
			model = env.tester.Update(t, model, nil, nil).Model.(*computeModel)
			assert.Equal(t, "HELLO WORLD!", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("Hello world!"),
				Valid:    true,
			}, model.Status)
		})

		env.tester.Refresh(model)
		assert.Equal(t, "WHAT'S UP?", model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     Hash("What's up?"),
			Valid:    true,
		}, model.Status)
	})
}

func TestComputeRehashInterval(t *testing.T) {
	testOperation(t, Compute(Computation{
		Name:   "Status",
		Model:  &computeModel{},
		Hasher: StringHasher("Input"),
		Computer: StringComputer("Input", "Output", func(ctx *Context, input string) (string, error) {
			return strings.ToUpper(input), nil
		}),
		Releaser: func(ctx *Context) error {
			ctx.Change("$set", "Output", "")
			return nil
		},
		RehashInterval: time.Millisecond,
	}), func(env operationTest) {
		model := env.tester.Insert(&computeModel{
			Base:  coal.B(),
			Input: "Hello world!",
		}).(*computeModel)

		/* first input */

		n, err := axe.AwaitJob(env.tester.Store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 3, n)

		env.tester.Refresh(model)
		assert.Equal(t, "HELLO WORLD!", model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     Hash("Hello world!"),
			Valid:    true,
		}, model.Status)

		/* rehash same */

		n, err = axe.AwaitJob(env.tester.Store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 2, n)

		/* rehash changed */

		before := model.Status.Updated

		model.Input = "What's up?"
		env.tester.Replace(model)

		n, err = axe.AwaitJob(env.tester.Store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 3, n)

		env.tester.Refresh(model)
		assert.Equal(t, "WHAT'S UP?", model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     Hash("What's up?"),
			Valid:    true,
		}, model.Status)
		assert.True(t, model.Status.Updated.After(before))
	})
}

func TestComputeRecomputeInterval(t *testing.T) {
	testOperation(t, Compute(Computation{
		Name:   "Status",
		Model:  &computeModel{},
		Hasher: StringHasher("Input"),
		Computer: StringComputer("Input", "Output", func(ctx *Context, input string) (string, error) {
			return strings.ToUpper(input), nil
		}),
		Releaser: func(ctx *Context) error {
			ctx.Change("$set", "Output", "")
			return nil
		},
		RecomputeInterval: time.Millisecond,
	}), func(env operationTest) {
		model := env.tester.Insert(&computeModel{
			Base:  coal.B(),
			Input: "Hello world!",
		}).(*computeModel)

		/* first input */

		n, err := axe.AwaitJob(env.tester.Store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 3, n)

		env.tester.Refresh(model)
		assert.Equal(t, "HELLO WORLD!", model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     Hash("Hello world!"),
			Valid:    true,
		}, model.Status)

		/* recompute same */

		updated := model.Status.Updated

		n, err = axe.AwaitJob(env.tester.Store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 3, n)

		env.tester.Refresh(model)
		assert.Equal(t, "HELLO WORLD!", model.Output)
		assert.Equal(t, &Status{
			Progress: 1,
			Updated:  model.Status.Updated,
			Hash:     Hash("Hello world!"),
			Valid:    true,
		}, model.Status)
		assert.True(t, model.Status.Updated.After(updated))
	})
}
