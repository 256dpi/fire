package torch

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, Compute(Computation{
			Name:   "Status",
			Model:  &computeModel{},
			Hasher: StringHasher("Input"),
			Computer: StringComputer("Input", "Output", func(ctx *Context, input string) (string, error) {
				return strings.ToUpper(input), nil
			}),
		}), func(env Env) {
			model := env.Insert(&computeModel{
				Base: coal.B(),
			}).(*computeModel)

			/* missing input */

			n, err := env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Zero(t, model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     "",
				Valid:    true,
			}, model.Status)
			assert.NotZero(t, model.Status.Updated)

			/* first input */

			oldUpdated := model.Status.Updated

			model.Input = "Hello world!"
			model.Status.Valid = false
			env.Replace(model)

			n, err = env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Equal(t, "HELLO WORLD!", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("Hello world!"),
				Valid:    true,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(oldUpdated))

			/* same input */

			oldOutput := model.Output
			oldStatus := *model.Status

			n, err = env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 0, n)

			env.Refresh(model)
			assert.NotNil(t, model.Output)
			assert.Equal(t, oldOutput, model.Output)
			assert.Equal(t, oldStatus, *model.Status)

			/* new input */

			oldUpdated = model.Status.Updated

			model.Input = "What's up?"
			env.Replace(model)

			n, err = env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 0, n)

			err = env.Process(model)
			assert.NoError(t, err)

			env.Refresh(model)
			assert.Equal(t, "WHAT'S UP?", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("What's up?"),
				Valid:    true,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(oldUpdated))

			/* invalid status */

			oldUpdated = model.Status.Updated

			model.Status.Valid = false
			env.Replace(model)

			err = env.Process(model)
			assert.NoError(t, err)

			env.Refresh(model)
			assert.Equal(t, "WHAT'S UP?", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("What's up?"),
				Valid:    true,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(oldUpdated))

			/* leftover input */

			oldUpdated = model.Status.Updated

			model.Input = ""
			env.Replace(model)

			err = env.Process(model)
			assert.NoError(t, err)

			env.Refresh(model)
			assert.Zero(t, model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     "",
				Valid:    true,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(oldUpdated))
		})
	})
}

func TestComputeProcess(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, Compute(Computation{
			Name:   "Status",
			Model:  &computeModel{},
			Hasher: StringHasher("Input"),
			Computer: StringComputer("Input", "Output", func(ctx *Context, input string) (string, error) {
				return strings.ToUpper(input), nil
			}),
		}), func(env Env) {
			var model *computeModel

			/* missing input */

			n := env.Await(t, 50*time.Millisecond, func() {
				model = env.Create(t, &computeModel{}, nil, nil).Model.(*computeModel)
				assert.Zero(t, model.Output)
				assert.Equal(t, &Status{
					Progress: 1,
					Updated:  model.Status.Updated,
					Hash:     "",
					Valid:    true,
				}, model.Status)
				assert.NotZero(t, model.Status.Updated)
			})
			assert.Equal(t, 0, n)

			/* first input */

			oldOutput := model.Output
			oldStatus := *model.Status

			model.Input = "Hello world!"
			n = env.Await(t, 0, func() {
				model = env.Update(t, model, nil, nil).Model.(*computeModel)
				assert.Equal(t, oldOutput, model.Output)
				assert.Equal(t, oldStatus, *model.Status)
			})
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Equal(t, "HELLO WORLD!", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("Hello world!"),
				Valid:    true,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(oldStatus.Updated))

			/* same input */

			oldOutput = model.Output

			n = env.Await(t, 50*time.Millisecond, func() {
				model = env.Update(t, model, nil, nil).Model.(*computeModel)
				assert.Equal(t, oldOutput, model.Output)
			})
			assert.Equal(t, 0, n)

			/* new input */

			oldOutput = model.Output
			oldStatus = *model.Status

			model.Input = "What's up?"
			n = env.Await(t, 0, func() {
				model = env.Update(t, model, nil, nil).Model.(*computeModel)
				assert.Equal(t, oldOutput, model.Output)
				assert.Equal(t, oldStatus, *model.Status)
			})
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Equal(t, "WHAT'S UP?", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("What's up?"),
				Valid:    true,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(oldStatus.Updated))

			/* invalid status */

			oldOutput = model.Output
			oldStatus = *model.Status
			oldStatus.Valid = false

			model.Status.Valid = false
			n = env.Await(t, 0, func() {
				model = env.Update(t, model, nil, nil).Model.(*computeModel)
				assert.Equal(t, oldOutput, model.Output)
				assert.Equal(t, oldStatus, *model.Status)
			})
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Equal(t, "WHAT'S UP?", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("What's up?"),
				Valid:    true,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(oldStatus.Updated))

			/* leftover input */

			oldOutput = model.Output
			oldStatus = *model.Status

			model.Input = ""
			n = env.Await(t, 0, func() {
				env.Update(t, model, nil, nil)
				assert.Equal(t, oldOutput, model.Output)
				assert.Equal(t, oldStatus, *model.Status)
			})
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Zero(t, model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     "",
				Valid:    true,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(oldStatus.Updated))
		})
	})
}

func TestComputeProgress(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, Compute(Computation{
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
		}), func(env Env) {
			var progress []float64
			stream := coal.Reconcile(env.Store, &computeModel{}, nil, func(model coal.Model) {
				progress = append(progress, model.(*computeModel).Status.Progress)
			}, func(model coal.Model) {
				progress = append(progress, model.(*computeModel).Status.Progress)
			}, nil, nil)
			defer stream.Close()

			var model *computeModel
			env.Await(t, 0, func() {
				model = env.Create(t, &computeModel{
					Input: "Hello world!",
				}, nil, nil).Model.(*computeModel)
			})

			env.Refresh(model)
			assert.Equal(t, "HELLO WORLD!", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("Hello world!"),
				Valid:    true,
			}, model.Status)
			assert.Equal(t, []float64{0, 0.25, 0.5, 0.75, 1}, progress)
		})
	})
}

func TestComputeReleaser(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, Compute(Computation{
			Name:   "Status",
			Model:  &computeModel{},
			Hasher: StringHasher("Input"),
			Computer: StringComputer("Input", "Output", func(ctx *Context, input string) (string, error) {
				return strings.ToUpper(input), nil
			}),
			Releaser: func(ctx *Context) error {
				time.Sleep(10 * time.Millisecond) // ensure time change
				ctx.Change("$set", "Output", "")
				return nil
			},
		}), func(env Env) {
			var model *computeModel

			/* first input */

			n := env.Await(t, 0, func() {
				model = env.Create(t, &computeModel{
					Input: "Hello world!",
				}, nil, nil).Model.(*computeModel)
			})
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Equal(t, "HELLO WORLD!", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("Hello world!"),
				Valid:    true,
			}, model.Status)

			/* new input */

			oldUpdated := model.Status.Updated

			model.Input = "What's up?"
			n = env.Await(t, 0, func() {
				model = env.Update(t, model, nil, nil).Model.(*computeModel)
				assert.Zero(t, model.Output)
				assert.Equal(t, &Status{
					Progress: 0,
					Updated:  model.Status.Updated,
					Hash:     "",
					Valid:    false,
				}, model.Status)
				assert.True(t, model.Status.Updated.After(oldUpdated))
			})
			assert.Equal(t, 1, n)

			oldUpdated = model.Status.Updated

			env.Refresh(model)
			assert.Equal(t, "WHAT'S UP?", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("What's up?"),
				Valid:    true,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(oldUpdated))

			/* leftover input */

			oldUpdated = model.Status.Updated

			model.Input = ""
			model = env.Update(t, model, nil, nil).Model.(*computeModel)
			assert.Zero(t, model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     "",
				Valid:    true,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(oldUpdated))
		})
	})
}

func TestComputeKeepOutdated(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, Compute(Computation{
			Name:   "Status",
			Model:  &computeModel{},
			Hasher: StringHasher("Input"),
			Computer: StringComputer("Input", "Output", func(ctx *Context, input string) (string, error) {
				return strings.ToUpper(input), nil
			}),
			Releaser: func(ctx *Context) error {
				time.Sleep(10 * time.Millisecond) // ensure time change
				ctx.Change("$set", "Output", "")
				return nil
			},
			KeepOutdated: true,
		}), func(env Env) {
			var model *computeModel

			/* first input */

			n := env.Await(t, 0, func() {
				model = env.Create(t, &computeModel{
					Input: "Hello world!",
				}, nil, nil).Model.(*computeModel)
			})
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Equal(t, "HELLO WORLD!", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("Hello world!"),
				Valid:    true,
			}, model.Status)

			/* new input */

			oldOutput := model.Output
			oldStatus := *model.Status

			model.Input = "What's up?"
			n = env.Await(t, 0, func() {
				model = env.Update(t, model, nil, nil).Model.(*computeModel)
				assert.Equal(t, oldOutput, model.Output)
				assert.Equal(t, oldStatus, *model.Status)
			})
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Equal(t, "WHAT'S UP?", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("What's up?"),
				Valid:    true,
			}, model.Status)

			/* leftover input */

			oldStatus = *model.Status

			model.Input = ""
			model = env.Update(t, model, nil, nil).Model.(*computeModel)
			assert.Zero(t, model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     "",
				Valid:    true,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(oldStatus.Updated))
		})
	})
}

func TestComputeRehashInterval(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, Compute(Computation{
			Name:   "Status",
			Model:  &computeModel{},
			Hasher: StringHasher("Input"),
			Computer: StringComputer("Input", "Output", func(ctx *Context, input string) (string, error) {
				return strings.ToUpper(input), nil
			}),
			RehashInterval: time.Millisecond,
		}), func(env Env) {
			model := env.Insert(&computeModel{
				Base:  coal.B(),
				Input: "Hello world!",
			}).(*computeModel)

			/* first input */

			n, err := env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Equal(t, "HELLO WORLD!", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("Hello world!"),
				Valid:    true,
			}, model.Status)

			/* rehash same */

			n, err = env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 0, n)

			/* rehash changed */

			before := model.Status.Updated

			model.Input = "What's up?"
			env.Replace(model)

			n, err = env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Equal(t, "WHAT'S UP?", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("What's up?"),
				Valid:    true,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(before))
		})
	})
}

func TestComputeRecomputeInterval(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, Compute(Computation{
			Name:   "Status",
			Model:  &computeModel{},
			Hasher: StringHasher("Input"),
			Computer: StringComputer("Input", "Output", func(ctx *Context, input string) (string, error) {
				return strings.ToUpper(input), nil
			}),
			RecomputeInterval: time.Millisecond,
		}), func(env Env) {
			model := env.Insert(&computeModel{
				Base:  coal.B(),
				Input: "Hello world!",
			}).(*computeModel)

			/* first input */

			n, err := env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Equal(t, "HELLO WORLD!", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("Hello world!"),
				Valid:    true,
			}, model.Status)

			/* recompute same */

			updated := model.Status.Updated

			n, err = env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Equal(t, "HELLO WORLD!", model.Output)
			assert.Equal(t, &Status{
				Progress: 1,
				Updated:  model.Status.Updated,
				Hash:     Hash("Hello world!"),
				Valid:    true,
			}, model.Status)
			assert.True(t, model.Status.Updated.After(updated))
		})
	})
}
