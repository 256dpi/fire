package torch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

type checkModel struct {
	coal.Base `json:"-" bson:",inline" coal:"check"`
	Counter   int
	Checked   *time.Time
	stick.NoValidation
}

func checkModelCheck() Check {
	return Check{
		Name:     "Checked",
		Model:    &checkModel{},
		Interval: time.Minute,
		Handler: func(ctx *Context) error {
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
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, CheckField(checkModelCheck()), func(env Env) {
			model := &checkModel{}

			n := env.Await(t, 0, func() {
				model = env.Create(t, model, nil, nil).Model.(*checkModel)
				assert.Equal(t, 0, model.Counter)
				assert.Nil(t, model.Checked)
			})
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Equal(t, 1, model.Counter)
			assert.NotNil(t, model.Checked)
			assert.NotZero(t, *model.Checked)
			assert.True(t, time.Since(*model.Checked) < time.Second)

			num, err := env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 0, num)

			/* clear */

			model.Checked = nil
			env.Replace(model)

			num, err = env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 1, num)

			env.Refresh(model)
			assert.Equal(t, 2, model.Counter)
			assert.NotNil(t, model.Checked)
			assert.NotZero(t, *model.Checked)
			assert.True(t, time.Since(*model.Checked) < time.Second)

			/* outdated */

			model.Checked = stick.P(time.Now().Add(-2 * time.Minute))
			env.Replace(model)

			num, err = env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 1, num)

			env.Refresh(model)
			assert.Equal(t, 3, model.Counter)
			assert.NotNil(t, model.Checked)
			assert.NotZero(t, *model.Checked)
			assert.True(t, time.Since(*model.Checked) < time.Second)
		})
	})
}

func TestCheckTag(t *testing.T) {
	withStore(t, func(t *testing.T, store *coal.Store) {
		Test(store, CheckTag(checkModelCheck()), func(env Env) {
			model := &checkModel{}

			n := env.Await(t, 0, func() {
				model = env.Create(t, model, nil, nil).Model.(*checkModel)
				assert.Equal(t, 0, model.Counter)
				assert.Nil(t, model.Checked)
			})
			assert.Equal(t, 1, n)

			env.Refresh(model)
			assert.Equal(t, 1, model.Counter)

			num, err := env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 0, num)

			/* clear */

			model.SetTag("Checked", nil, time.Now())
			env.Replace(model)

			num, err = env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 1, num)

			env.Refresh(model)
			assert.Equal(t, 2, model.Counter)

			/* outdated */

			model.SetTag("Checked", time.Now().Add(-2*time.Minute), time.Now())
			env.Replace(model)

			num, err = env.Scan()
			assert.NoError(t, err)
			assert.Equal(t, 1, num)

			env.Refresh(model)
			assert.Equal(t, 3, model.Counter)
		})
	})
}
