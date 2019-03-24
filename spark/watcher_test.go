package spark

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestWatcherWebSockets(t *testing.T) {
	tester.Clean()

	watcher := NewWatcher()
	watcher.Reporter = func(err error) { panic(err) }
	watcher.Add(&Stream{
		Model: &itemModel{},
		Store: tester.Store,
	})

	group := tester.Assign("", &fire.Controller{
		Model: &itemModel{},
		Store: tester.Store,
	})
	group.Handle("watch", &fire.GroupAction{
		Action: watcher.Action(),
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

	tester.Delete(itm)

	typ, bytes, err = ws.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, typ)
	assert.JSONEq(t, `{
		"items": {
			"`+itm.ID().Hex()+`": "deleted"
		}
	}`, string(bytes))
}

func TestWatcherSSE(t *testing.T) {
	tester.Clean()

	watcher := NewWatcher()
	watcher.Reporter = func(err error) { panic(err) }
	watcher.Add(&Stream{
		Model: &itemModel{},
		Store: tester.Store,
	})

	group := tester.Assign("", &fire.Controller{
		Model: &itemModel{},
		Store: tester.Store,
	})
	group.Handle("watch", &fire.GroupAction{
		Action: watcher.Action(),
	})

	rec := newResponseRecorder()
	data := base64.StdEncoding.EncodeToString([]byte(`{ "items": { "state": true } }`))
	req := httptest.NewRequest("GET", "/watch?s=items&d="+data, nil)

	itm := coal.Init(&itemModel{
		Bar: "bar",
	}).(*itemModel)

	go func() {
		time.Sleep(100 * time.Millisecond)

		tester.Save(itm)

		itm.Foo = "bar"
		tester.Update(itm)

		tester.Delete(itm)

		time.Sleep(100 * time.Millisecond)

		rec.Close()
	}()

	group.Endpoint("").ServeHTTP(rec, req)

	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, true, rec.Flushed)
	assert.Equal(t, []string{
		`data: {"items":{"` + itm.ID().Hex() + `":"created"}}`,
		`data: {"items":{"` + itm.ID().Hex() + `":"updated"}}`,
		`data: {"items":{"` + itm.ID().Hex() + `":"deleted"}}`,
	}, strings.Split(strings.TrimSpace(rec.Body.String()), "\n\n"))
}
