package spark

import (
	"net/http"
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

	watcher := NewWatcher()
	watcher.Add(&Stream{
		Model: &itemModel{},
		Store: tester.Store,
	})
	watcher.Run()

	group := tester.Assign("", &fire.Controller{
		Model: &itemModel{},
		Store: tester.Store,
	})
	group.Handle("watch", watcher.Action())

	/* run server and create client */

	server := &http.Server{Addr: "0.0.0.0:1234", Handler: tester.Handler}
	go func() { _ = server.ListenAndServe() }()
	defer server.Close()

	time.Sleep(10 * time.Millisecond)

	ws, _, err := websocket.DefaultDialer.Dial("ws://0.0.0.0:1234/watch", nil)
	assert.NoError(t, err)
	assert.NotNil(t, ws)

	defer ws.Close()

	/* subscribe */

	err = ws.WriteMessage(websocket.TextMessage, []byte(`{
		"subscribe": {
			"items": {}
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
			"`+itm.ID().Hex()+`": "created"
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
			"`+itm.ID().Hex()+`": "updated"
		}
	}`, string(bytes))

	/* delete model */

	tester.Delete(item)

	typ, bytes, err = ws.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, typ)
	assert.JSONEq(t, `{
		"items": {
			"`+item.ID().Hex()+`": "deleted"
		}
	}`, string(bytes))
}
