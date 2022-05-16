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

		post, doc, err := client.Create(&testModel{
			String: "string",
		})
		assert.NoError(t, err)
		assert.Equal(t, &testModel{
			Base:   *post.GetBase(),
			String: "string",
		}, post)
		assert.NotNil(t, doc)

		list, doc, err = client.List(&testModel{})
		assert.NoError(t, err)
		assert.Equal(t, []coal.Model{
			&testModel{
				Base:   *post.GetBase(),
				String: "string",
			},
		}, list)
		assert.NotNil(t, doc)

		post.(*testModel).Bool = true
		post, doc, err = client.Update(post)
		assert.NoError(t, err)
		assert.Equal(t, &testModel{
			Base:   *post.GetBase(),
			String: "string",
			Bool:   true,
		}, post)
		assert.NotNil(t, doc)

		post, doc, err = client.Find(post)
		assert.NoError(t, err)
		assert.Equal(t, &testModel{
			Base:   *post.GetBase(),
			String: "string",
			Bool:   true,
		}, post)
		assert.NotNil(t, doc)

		err = client.Delete(post)
		assert.NoError(t, err)

		post, doc, err = client.Find(post)
		assert.Error(t, err)
		assert.Equal(t, jsonapi.NotFound("resource not found"), err)
		assert.Nil(t, post)
		assert.NotNil(t, doc)
	})
}
