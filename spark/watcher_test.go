package spark

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

var tester = fire.NewTester(
	coal.MustCreateStore("mongodb://0.0.0.0:27017/test-fire-spark"),
	&itemModel{},
)

type itemModel struct {
	coal.Base `json:"-" bson:",inline" coal:"items"`
	Foo       string
	Bar       string
}

func TestWatcher(t *testing.T) {
	tester.Clean()

	item := tester.Save(&itemModel{}).(*itemModel)

	policy := DefaultPolicy("")

	watcher := NewWatcher(tester.Store, policy)
	watcher.Watch(&itemModel{})

	group := tester.Assign("", &fire.Controller{
		Model: &itemModel{},
		Store: tester.Store,
		CollectionActions: fire.M{
			"watch": watcher.Collection(func(ctx *fire.Context) map[string]interface{} {
				// only forward changes for "bar" models
				return map[string]interface{}{
					coal.F(&itemModel{}, "Bar"): "bar",
				}
			}),
		},
		ResourceActions: fire.M{
			"watch": watcher.Resource(),
		},
	})

	group.Handle("watch", watcher.GroupAction())

	/* get watch tokens */

	var collectionWatchToken string
	tester.Request("GET", "items/watch", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))

		claims, ok, err := policy.ParseToken(r.Body.String())
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, "items", claims.Subject)
		assert.Equal(t, "", claims.Id)
		assert.Equal(t, map[string]interface{}{
			"bar": "bar",
		}, claims.Data)

		collectionWatchToken = r.Body.String()
	})

	var resourceWatchToken string
	tester.Request("GET", "items/"+item.ID().Hex()+"/watch", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))

		claims, ok, err := policy.ParseToken(r.Body.String())
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, "items", claims.Subject)
		assert.Equal(t, item.ID().Hex(), claims.Id)
		assert.Equal(t, map[string]interface{}(nil), claims.Data)

		resourceWatchToken = r.Body.String()
	})

	/* run server and create client */

	server := &http.Server{Addr: "0.0.0.0:1234", Handler: tester.Handler}
	go func() { _ = server.ListenAndServe() }()
	defer server.Close()

	time.Sleep(10 * time.Millisecond)

	ws, _, err := websocket.DefaultDialer.Dial("ws://0.0.0.0:1234/watch", nil)
	assert.NoError(t, err)
	assert.NotNil(t, ws)

	defer ws.Close()

	/* subscribe watch tokens */

	err = ws.WriteMessage(websocket.TextMessage, []byte(`{
		"subscribe": {
			"1": "`+collectionWatchToken+`"
		}
	}`))
	assert.NoError(t, err)

	err = ws.WriteMessage(websocket.TextMessage, []byte(`{
		"subscribe": {
			"2": "`+resourceWatchToken+`"
		}
	}`))
	assert.NoError(t, err)

	/* create model */

	itm := coal.Init(&itemModel{
		Bar: "bar",
	}).(*itemModel)

	tester.Save(itm)

	typ, bytes, err := ws.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, typ)
	assert.JSONEq(t, `{
		"items": {
			"`+itm.ID().Hex()+`": "create"
		}
	}`, string(bytes))

	/* update model */

	itm.Foo = "bar"

	tester.Update(itm)

	typ, bytes, err = ws.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, typ)
	assert.JSONEq(t, `{
		"items": {
			"`+itm.ID().Hex()+`": "update"
		}
	}`, string(bytes))

	/* unsubscribe watch token */

	err = ws.WriteMessage(websocket.TextMessage, []byte(`{
		"unsubscribe": ["1"]
	}`))
	assert.NoError(t, err)

	itm.Foo = "baz"

	tester.Update(itm)

	/* delete model */

	tester.Delete(item)

	typ, bytes, err = ws.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, typ)
	assert.JSONEq(t, `{
		"items": {
			"`+item.ID().Hex()+`": "delete"
		}
	}`, string(bytes))
}
