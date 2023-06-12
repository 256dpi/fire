package torch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

type testModel struct {
	coal.Base `json:"-" bson:",inline" coal:"test"`
	Input     int
	Output    int
	stick.NoValidation
}

func testModelOp() *Operation {
	return &Operation{
		Name:  "foo",
		Model: &testModel{},
		Processor: func(ctx *Context) error {
			model := ctx.Model.(*testModel)
			if model.Input == -1 {
				return axe.E("invalid input", false)
			} else if model.Input == -2 && (ctx.Sync || ctx.Context.(*axe.Context).Attempt == 1) {
				ctx.Defer = true
				return nil
			}
			ctx.Change("$set", "Output", model.Input*2)
			return nil
		},
	}
}

func TestReactorCheck(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, testModelOp(), func(env Env) {
			model := &testModel{Base: coal.B(), Input: 7}
			env.Insert(model)

			num := env.Await(t, 0, func() {
				err := env.Reactor.Check(nil, model)
				assert.NoError(t, err)
			})
			assert.Equal(t, 1, num)

			env.Refresh(model)
			assert.Equal(t, 14, model.Output)
		})
	})
}

func TestReactorCheckFilter(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, testModelOp(), func(env Env) {
			env.Operation.Filter = func(model coal.Model) bool {
				return model.(*testModel).Input%7 != 0
			}

			model := &testModel{Base: coal.B(), Input: 7}
			env.Insert(model)

			num := env.Await(t, 50*time.Millisecond, func() {
				err := env.Reactor.Check(nil, model)
				assert.NoError(t, err)
			})
			assert.Equal(t, 0, num)

			env.Refresh(model)
			assert.Equal(t, 0, model.Output)

			model.Input = 9
			env.Replace(model)

			num = env.Await(t, 0, func() {
				err := env.Reactor.Check(nil, model)
				assert.NoError(t, err)
			})
			assert.Equal(t, 1, num)

			env.Refresh(model)
			assert.Equal(t, 18, model.Output)
		})
	})
}

func TestReactorCheckError(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, testModelOp(), func(env Env) {
			model := &testModel{Base: coal.B(), Input: -1}
			env.Insert(model)

			num, err := axe.Await(env.Store, 0, func() error {
				return env.Reactor.Check(nil, model)
			})
			assert.Error(t, err)
			assert.Equal(t, 1, num)
			assert.Equal(t, "failed: invalid input", err.Error())
		})
	})
}

func TestReactorCheckDefer(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, testModelOp(), func(env Env) {
			env.Operation.Sync = true

			model := &testModel{Base: coal.B(), Input: -2}
			env.Insert(model)

			num, err := axe.Await(env.Store, 0, func() error {
				return env.Reactor.Check(nil, model)
			})
			assert.Error(t, err)
			assert.Equal(t, 1, num)
			assert.Equal(t, "failed: deferred", err.Error())

			num = env.Await(t, 0)
			assert.Equal(t, 1, num)

			env.Refresh(model)
			assert.Equal(t, -4, model.Output)
		})
	})
}

func TestReactorScan(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, testModelOp(), func(env Env) {
			env.Operation.Query = func() bson.M {
				return bson.M{
					"Output": 0,
				}
			}

			model := &testModel{Base: coal.B(), Input: 7}
			env.Insert(model)

			num, err := env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 1, num)

			env.Refresh(model)
			assert.Equal(t, 14, model.Output)

			num, err = env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 0, num)
		})
	})
}

func TestReactorScanFilter(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, testModelOp(), func(env Env) {
			env.Operation.Query = func() bson.M {
				return bson.M{
					"Output": 0,
				}
			}
			env.Operation.Filter = func(model coal.Model) bool {
				return model.(*testModel).Input%7 != 0
			}

			model := &testModel{Base: coal.B(), Input: 7}
			env.Insert(model)

			num, err := env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 0, num)

			env.Refresh(model)
			assert.Equal(t, 0, model.Output)

			model.Input = 9
			env.Replace(model)

			num, err = env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 1, num)

			env.Refresh(model)
			assert.Equal(t, 18, model.Output)
		})
	})
}

func TestReactorProcessFilter(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, testModelOp(), func(env Env) {
			env.Operation.Query = func() bson.M {
				return bson.M{
					"Output": 0,
				}
			}
			env.Operation.Filter = func(model coal.Model) bool {
				return model.(*testModel).Input%7 != 0
			}

			model := &testModel{Base: coal.B(), Input: 7}
			model.SetTag("torch/Reactor/foo", 1, time.Now().Add(time.Hour))
			env.Insert(model)

			err := env.Process(model)
			assert.NoError(t, err)

			env.Refresh(model)
			assert.Equal(t, 0, model.Output)
			assert.Equal(t, int32(0), model.GetTag("torch/Reactor/foo"))
		})
	})
}

func TestReactorModifierAsync(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, testModelOp(), func(env Env) {
			model := env.Create(t, &testModel{
				Input: 7,
			}, nil, nil).Model.(*testModel)
			assert.NotNil(t, model)
			assert.Equal(t, 0, model.Output)

			num := env.Await(t, 0)
			assert.Equal(t, 1, num)

			model = env.Find(t, model, nil).Model.(*testModel)
			assert.NotNil(t, model)
			assert.Equal(t, 14, model.Output)

			model.Input = 17
			model = env.Update(t, model, nil, nil).Model.(*testModel)
			assert.NotNil(t, model)
			assert.Equal(t, 14, model.Output)

			num = env.Await(t, 0)
			assert.Equal(t, 1, num)

			model = env.Find(t, model, nil).Model.(*testModel)
			assert.NotNil(t, model)
			assert.Equal(t, 34, model.Output)

			env.Delete(t, model, nil)

			num = env.Await(t, 50*time.Millisecond)
			assert.Equal(t, 0, num)
		})
	})
}

func TestReactorModifierSync(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, testModelOp(), func(env Env) {
			env.Operation.Sync = true

			model := env.Create(t, &testModel{
				Input: 7,
			}, nil, nil).Model.(*testModel)
			assert.NotNil(t, model)
			assert.Equal(t, 14, model.Output)

			num := env.Await(t, 50*time.Millisecond)
			assert.Equal(t, 0, num)

			model = env.Find(t, model, nil).Model.(*testModel)
			assert.NotNil(t, model)
			assert.Equal(t, 14, model.Output)

			model.Input = 17
			model = env.Update(t, model, nil, nil).Model.(*testModel)
			assert.NotNil(t, model)
			assert.Equal(t, 34, model.Output)

			num = env.Await(t, 50*time.Millisecond)
			assert.Equal(t, 0, num)

			model = env.Find(t, model, nil).Model.(*testModel)
			assert.NotNil(t, model)
			assert.Equal(t, 34, model.Output)

			env.Delete(t, model, nil)

			num = env.Await(t, 50*time.Millisecond)
			assert.Equal(t, 0, num)
		})
	})
}

func TestReactorModifierIdempotence(t *testing.T) {
	// the reactor will not queue jobs for the subsequent updates, but the
	// process of the insert will observe all updates

	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, testModelOp(), func(env Env) {
			model := &testModel{Input: 7}

			num := env.Await(t, 0, func() {
				model = env.Create(t, model, nil, nil).Model.(*testModel)
				assert.NotNil(t, model)

				for i := 0; i < 2; i++ {
					model.Input *= 2
					env.Update(t, model, nil, nil)
				}
			})
			assert.Equal(t, 1, num)

			env.Refresh(model)
			assert.NotNil(t, model)
			assert.Equal(t, 28, model.Input)
			assert.Equal(t, 56, model.Output)

			num, err := env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 0, num)
		})
	})
}

func TestReactorModifierConcurrency(t *testing.T) {
	// the reactor will queue and execute after the creation and only tag the
	// models as outstanding on the subsequent updates. the final execution is
	// triggered by the scan

	withStore(t, func(t *testing.T, store *coal.Store) {
		paused := make(chan struct{}, 3)
		resumed := make(chan struct{})

		op := &Operation{
			Name:  "foo",
			Model: &testModel{},
			Processor: func(ctx *Context) error {
				paused <- struct{}{}
				<-resumed
				model := ctx.Model.(*testModel)
				ctx.Change("$set", "Output", model.Input*2)
				return nil
			},
		}

		Test(store, op, func(env Env) {
			model := &testModel{Input: 7}

			num := env.Await(t, 0, func() {
				model = env.Create(t, model, nil, nil).Model.(*testModel)
				assert.NotNil(t, model)

				<-paused

				for i := 0; i < 2; i++ {
					model = env.Find(t, model, nil).Model.(*testModel)
					model.Input *= 2
					env.Update(t, model, nil, nil)
				}

				close(resumed)
			})
			assert.Equal(t, 1, num)

			env.Refresh(model)
			assert.NotNil(t, model)
			assert.NotZero(t, model.GetTag("torch/Reactor/foo").(int32))

			num, err := env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 1, num)

			env.Refresh(model)
			assert.NotNil(t, model)
			assert.Equal(t, 28, model.Input)
			assert.Equal(t, 56, model.Output)
		})
	})
}
