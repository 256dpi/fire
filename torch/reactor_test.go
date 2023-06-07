package torch

import (
	"testing"
	"time"

	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/roast"
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
		Process: func(ctx *Context) error {
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

type operationTest struct {
	store     *coal.Store
	queue     *axe.Queue
	reactor   *Reactor
	operation *Operation
	tester    *roast.Tester
}

func TestCheck(t *testing.T) {
	testOperation(t, testModelOp(), func(env operationTest) {
		model := &testModel{Base: coal.B(), Input: 7}
		env.tester.Insert(model)

		num, err := axe.Await(env.store, 0, func() error {
			return env.reactor.Check(nil, model)
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, num)

		env.tester.Refresh(model)
		assert.Equal(t, 14, model.Output)
	})
}

func TestCheckFilter(t *testing.T) {
	testOperation(t, testModelOp(), func(env operationTest) {
		env.operation.Filter = func(model coal.Model) bool {
			return model.(*testModel).Input%7 != 0
		}

		model := &testModel{Base: coal.B(), Input: 7}
		env.tester.Insert(model)

		num, err := axe.Await(env.store, 50*time.Millisecond, func() error {
			return env.reactor.Check(nil, model)
		})
		assert.NoError(t, err)
		assert.Equal(t, 0, num)

		env.tester.Refresh(model)
		assert.Equal(t, 0, model.Output)

		model.Input = 9
		env.tester.Replace(model)

		num, err = axe.Await(env.store, 0, func() error {
			return env.reactor.Check(nil, model)
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, num)

		env.tester.Refresh(model)
		assert.Equal(t, 18, model.Output)
	})
}

func TestCheckError(t *testing.T) {
	testOperation(t, testModelOp(), func(env operationTest) {
		model := &testModel{Base: coal.B(), Input: -1}
		env.tester.Insert(model)

		num, err := axe.Await(env.store, 0, func() error {
			return env.reactor.Check(nil, model)
		})
		assert.Error(t, err)
		assert.Equal(t, 1, num)
		assert.Equal(t, "failed: invalid input", err.Error())
	})
}

func TestCheckDefer(t *testing.T) {
	testOperation(t, testModelOp(), func(env operationTest) {
		env.operation.Sync = true

		model := &testModel{Base: coal.B(), Input: -2}
		env.tester.Insert(model)

		num, err := axe.Await(env.store, 0, func() error {
			return env.reactor.Check(nil, model)
		})
		assert.Error(t, err)
		assert.Equal(t, 1, num)
		assert.Equal(t, "failed: deferred", err.Error())

		num, err = axe.Await(env.store, time.Minute)
		assert.NoError(t, err)
		assert.Equal(t, 1, num)

		env.tester.Refresh(model)
		assert.Equal(t, -4, model.Output)
	})
}

func TestScan(t *testing.T) {
	testOperation(t, testModelOp(), func(env operationTest) {
		env.operation.Query = func() bson.M {
			return bson.M{
				"Output": 0,
			}
		}

		model := &testModel{Base: coal.B(), Input: 7}
		env.tester.Insert(model)

		num, err := axe.AwaitJob(env.store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 3, num)

		env.tester.Refresh(model)
		assert.Equal(t, 14, model.Output)

		num, err = axe.AwaitJob(env.store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 2, num)
	})
}

func TestScanFilter(t *testing.T) {
	testOperation(t, testModelOp(), func(env operationTest) {
		env.operation.Query = func() bson.M {
			return bson.M{
				"Output": 0,
			}
		}
		env.operation.Filter = func(model coal.Model) bool {
			return model.(*testModel).Input%7 != 0
		}

		model := &testModel{Base: coal.B(), Input: 7}
		env.tester.Insert(model)

		num, err := axe.AwaitJob(env.store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 2, num)

		env.tester.Refresh(model)
		assert.Equal(t, 0, model.Output)

		model.Input = 9
		env.tester.Replace(model)

		num, err = axe.AwaitJob(env.store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 3, num)

		env.tester.Refresh(model)
		assert.Equal(t, 18, model.Output)
	})
}

func TestModifierAsync(t *testing.T) {
	testOperation(t, testModelOp(), func(env operationTest) {
		model := env.tester.Create(t, &testModel{
			Input: 7,
		}, nil, nil).Model.(*testModel)
		assert.NotNil(t, model)
		assert.Equal(t, 0, model.Output)

		num, err := axe.Await(env.store, 0)
		assert.NoError(t, err)
		assert.Equal(t, 1, num)

		model = env.tester.Find(t, model, nil).Model.(*testModel)
		assert.NotNil(t, model)
		assert.Equal(t, 14, model.Output)

		model.Input = 17
		model = env.tester.Update(t, model, nil, nil).Model.(*testModel)
		assert.NotNil(t, model)
		assert.Equal(t, 14, model.Output)

		num, err = axe.Await(env.store, 0)
		assert.NoError(t, err)
		assert.Equal(t, 1, num)

		model = env.tester.Find(t, model, nil).Model.(*testModel)
		assert.NotNil(t, model)
		assert.Equal(t, 34, model.Output)

		env.tester.Delete(t, model, nil)

		num, err = axe.Await(env.store, 50*time.Millisecond)
		assert.NoError(t, err)
		assert.Equal(t, 0, num)
	})
}

func TestModifierSync(t *testing.T) {
	testOperation(t, testModelOp(), func(env operationTest) {
		env.operation.Sync = true

		model := env.tester.Create(t, &testModel{
			Input: 7,
		}, nil, nil).Model.(*testModel)
		assert.NotNil(t, model)
		assert.Equal(t, 14, model.Output)

		num, err := axe.Await(env.store, 50*time.Millisecond)
		assert.NoError(t, err)
		assert.Equal(t, 0, num)

		model = env.tester.Find(t, model, nil).Model.(*testModel)
		assert.NotNil(t, model)
		assert.Equal(t, 14, model.Output)

		model.Input = 17
		model = env.tester.Update(t, model, nil, nil).Model.(*testModel)
		assert.NotNil(t, model)
		assert.Equal(t, 34, model.Output)

		num, err = axe.Await(env.store, 50*time.Millisecond)
		assert.NoError(t, err)
		assert.Equal(t, 0, num)

		model = env.tester.Find(t, model, nil).Model.(*testModel)
		assert.NotNil(t, model)
		assert.Equal(t, 34, model.Output)

		env.tester.Delete(t, model, nil)

		num, err = axe.Await(env.store, 50*time.Millisecond)
		assert.NoError(t, err)
		assert.Equal(t, 0, num)
	})
}

func TestModifierIdempotence(t *testing.T) {
	// the reactor will not queue jobs for the subsequent updates, but the
	// process of the insert will observe all updates

	testOperation(t, testModelOp(), func(env operationTest) {
		model := &testModel{Input: 7}

		num, err := axe.Await(env.store, 0, func() error {
			model = env.tester.Create(t, model, nil, nil).Model.(*testModel)
			assert.NotNil(t, model)

			for i := 0; i < 2; i++ {
				model.Input *= 2
				env.tester.Update(t, model, nil, nil)
			}

			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, num)

		env.tester.Refresh(model)
		assert.NotNil(t, model)
		assert.Equal(t, 28, model.Input)
		assert.Equal(t, 56, model.Output)

		num, err = axe.AwaitJob(env.store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 2, num)
	})
}

func TestModifierConcurrency(t *testing.T) {
	// the reactor will queue and execute after the creation and only tag the
	// models as outstanding on the subsequent updates. the final execution is
	// triggered by the scan

	testOperation(t, testModelOp(), func(env operationTest) {
		model := &testModel{Input: 7}

		num, err := axe.Await(env.store, 0, func() error {
			model = env.tester.Create(t, model, nil, nil).Model.(*testModel)
			assert.NotNil(t, model)

			for i := 0; i < 2; i++ {
				time.Sleep(50 * time.Millisecond)

				model = env.tester.Find(t, model, nil).Model.(*testModel)
				model.Input *= 2
				env.tester.Update(t, model, nil, nil)
			}

			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, num)

		env.tester.Refresh(model)
		assert.NotNil(t, model)
		assert.Equal(t, 28, model.Input)
		assert.Equal(t, 28, model.Output)

		num, err = axe.AwaitJob(env.store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 3, num)

		env.tester.Refresh(model)
		assert.NotNil(t, model)
		assert.Equal(t, 28, model.Input)
		assert.Equal(t, 56, model.Output)
	})
}

func testOperation(t *testing.T, operation *Operation, fn func(env operationTest)) {
	withTester(t, func(t *testing.T, store *coal.Store) {
		queue := axe.NewQueue(axe.Options{
			Store:    store,
			Reporter: xo.Panic,
		})

		reactor := NewReactor(store, queue)
		reactor.Add(operation)

		task := reactor.ScanTask()
		task.Periodicity = 0
		task.PeriodicJob = axe.Blueprint{}
		queue.Add(task)

		queue.Add(reactor.ProcessTask())

		queue.Run()
		defer queue.Close()

		group := fire.NewGroup(xo.Panic)

		group.Add(&fire.Controller{
			Store: store,
			Model: operation.Model,
			Modifiers: []*fire.Callback{
				reactor.Modifier(),
			},
		})

		tester := roast.NewTester(roast.Config{
			Store:   store,
			Models:  []coal.Model{operation.Model},
			Handler: group.Endpoint(""),
		})

		fn(operationTest{
			store:     store,
			queue:     queue,
			reactor:   reactor,
			operation: operation,
			tester:    tester,
		})
	})
}
