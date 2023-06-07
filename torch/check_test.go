package torch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

type checkModel struct {
	coal.Base `json:"-" bson:",inline" coal:"check"`
	Counter   int
	Checked   *time.Time
	stick.NoValidation
}

func checkModelCheck() Check[*checkModel] {
	return Check[*checkModel]{
		Name:     "Checked",
		Interval: time.Minute,
		Handler: func(ctx *Context, model *checkModel) error {
			ctx.Change("$inc", "Counter", 1)
			return nil
		},
	}
}

func TestCheckDeadline(t *testing.T) {
	check := checkModelCheck()
	check.Jitter = 0.5
	for i := 0; i < 100; i++ {
		dl := check.Deadline()
		assert.True(t, time.Since(dl) <= time.Minute)
		assert.True(t, time.Since(dl) >= time.Minute/2)
	}
}

func TestCheckField(t *testing.T) {
	testOperation(t, CheckField(checkModelCheck()), func(env operationTest) {
		model := &checkModel{}

		n := env.tester.Await(t, 0, func() {
			model = env.tester.Create(t, model, nil, nil).Model.(*checkModel)
		})
		assert.Equal(t, 1, n)
		assert.Equal(t, 0, model.Counter)
		assert.Nil(t, model.Checked)

		env.tester.Refresh(model)
		assert.Equal(t, 1, model.Counter)
		assert.NotNil(t, model.Checked)
		assert.NotZero(t, *model.Checked)
		assert.True(t, time.Since(*model.Checked) < time.Second)

		num, err := axe.AwaitJob(env.store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 2, num)

		/* clear */

		model.Checked = nil
		env.tester.Replace(model)

		num, err = axe.AwaitJob(env.store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 3, num)

		env.tester.Refresh(model)
		assert.Equal(t, 2, model.Counter)
		assert.NotNil(t, model.Checked)
		assert.NotZero(t, *model.Checked)
		assert.True(t, time.Since(*model.Checked) < time.Second)

		/* outdated */

		model.Checked = stick.P(time.Now().Add(-2 * time.Minute))
		env.tester.Replace(model)

		num, err = axe.AwaitJob(env.store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 3, num)

		env.tester.Refresh(model)
		assert.Equal(t, 3, model.Counter)
		assert.NotNil(t, model.Checked)
		assert.NotZero(t, *model.Checked)
		assert.True(t, time.Since(*model.Checked) < time.Second)
	})
}

func TestCheckTag(t *testing.T) {
	testOperation(t, CheckTag(checkModelCheck()), func(env operationTest) {
		model := &checkModel{}

		n := env.tester.Await(t, 0, func() {
			model = env.tester.Create(t, model, nil, nil).Model.(*checkModel)
		})
		assert.Equal(t, 1, n)
		assert.Equal(t, 0, model.Counter)
		assert.Nil(t, model.Checked)

		env.tester.Refresh(model)
		assert.Equal(t, 1, model.Counter)

		num, err := axe.AwaitJob(env.store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 2, num)

		/* clear */

		model.GetBase().SetTag("Checked", nil, time.Now())
		env.tester.Replace(model)

		num, err = axe.AwaitJob(env.store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 3, num)

		env.tester.Refresh(model)
		assert.Equal(t, 2, model.Counter)

		/* outdated */

		model.GetBase().SetTag("Checked", time.Now().Add(-2*time.Minute), time.Now())
		env.tester.Replace(model)

		num, err = axe.AwaitJob(env.store, 0, NewScanJob(""))
		assert.NoError(t, err)
		assert.Equal(t, 3, num)

		env.tester.Refresh(model)
		assert.Equal(t, 3, model.Counter)
	})
}
