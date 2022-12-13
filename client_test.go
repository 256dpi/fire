package fire

import (
	"net/http"
	"testing"

	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/serve"
	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
)

func TestClient(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		group := NewGroup(xo.Panic)
		group.Add(&Controller{
			Store: tester.Store,
			Model: &testModel{},
		})

		client := NewClient(jsonapi.NewClientWithClient(jsonapi.ClientConfig{}, &http.Client{
			Transport: serve.Local(group.Endpoint("")),
		}))

		list, doc, err := client.List(&testModel{})
		assert.NoError(t, err)
		assert.Empty(t, list)
		assert.NotNil(t, doc)

		model, doc, err := client.Create(&testModel{
			String: "string",
		})
		assert.NoError(t, err)
		assert.Equal(t, &testModel{
			Base:   *model.GetBase(),
			String: "string",
		}, model)
		assert.NotNil(t, doc)

		list, doc, err = client.List(&testModel{})
		assert.NoError(t, err)
		assert.Equal(t, []coal.Model{
			&testModel{
				Base:   *model.GetBase(),
				String: "string",
			},
		}, list)
		assert.NotNil(t, doc)

		model.(*testModel).Bool = true
		model, doc, err = client.Update(model)
		assert.NoError(t, err)
		assert.Equal(t, &testModel{
			Base:   *model.GetBase(),
			String: "string",
			Bool:   true,
		}, model)
		assert.NotNil(t, doc)

		model, doc, err = client.Find(model)
		assert.NoError(t, err)
		assert.Equal(t, &testModel{
			Base:   *model.GetBase(),
			String: "string",
			Bool:   true,
		}, model)
		assert.NotNil(t, doc)

		err = client.Delete(model)
		assert.NoError(t, err)

		model, doc, err = client.Find(model)
		assert.Error(t, err)
		assert.Equal(t, jsonapi.NotFound("resource not found"), err)
		assert.Nil(t, model)
		assert.NotNil(t, doc)

		list, doc, err = client.List(&testModel{})
		assert.NoError(t, err)
		assert.Nil(t, list)
		assert.NotNil(t, doc)
	})
}

func TestModelClient(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		group := NewGroup(xo.Panic)
		group.Add(&Controller{
			Store: tester.Store,
			Model: &testModel{},
		})

		client := ClientFor[*testModel](NewClient(jsonapi.NewClientWithClient(jsonapi.ClientConfig{}, &http.Client{
			Transport: serve.Local(group.Endpoint("")),
		})))

		list, doc, err := client.List()
		assert.NoError(t, err)
		assert.Empty(t, list)
		assert.NotNil(t, doc)

		model, doc, err := client.Create(&testModel{
			String: "string",
		})
		assert.NoError(t, err)
		assert.Equal(t, &testModel{
			Base:   *model.GetBase(),
			String: "string",
		}, model)
		assert.NotNil(t, doc)

		list, doc, err = client.List()
		assert.NoError(t, err)
		assert.Equal(t, []*testModel{
			{
				Base:   *model.GetBase(),
				String: "string",
			},
		}, list)
		assert.NotNil(t, doc)

		model.Bool = true
		model, doc, err = client.Update(model)
		assert.NoError(t, err)
		assert.Equal(t, &testModel{
			Base:   *model.GetBase(),
			String: "string",
			Bool:   true,
		}, model)
		assert.NotNil(t, doc)

		model, doc, err = client.Find(model.ID())
		assert.NoError(t, err)
		assert.Equal(t, &testModel{
			Base:   *model.GetBase(),
			String: "string",
			Bool:   true,
		}, model)
		assert.NotNil(t, doc)

		err = client.Delete(model.ID())
		assert.NoError(t, err)

		model, doc, err = client.Find(model.ID())
		assert.Error(t, err)
		assert.Equal(t, jsonapi.NotFound("resource not found"), err)
		assert.Nil(t, model)
		assert.NotNil(t, doc)

		list, doc, err = client.List()
		assert.NoError(t, err)
		assert.Equal(t, []*testModel{}, list)
		assert.NotNil(t, doc)
	})
}
