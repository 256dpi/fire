package spark

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func panicReporter(err error) {
	panic(err)
}

func TestWatcherWebSockets(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		watcher := NewWatcher(panicReporter)
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

		time.Sleep(100 * time.Millisecond)

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

		time.Sleep(100 * time.Millisecond)

		/* create model */

		itm := tester.Insert(&itemModel{
			Bar: "bar",
		}).(*itemModel)

		_ = ws.SetReadDeadline(time.Now().Add(time.Minute))
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
		tester.Replace(itm)

		_ = ws.SetReadDeadline(time.Now().Add(time.Minute))
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

		_ = ws.SetReadDeadline(time.Now().Add(time.Minute))
		typ, bytes, err = ws.ReadMessage()
		assert.NoError(t, err)
		assert.Equal(t, websocket.TextMessage, typ)
		assert.JSONEq(t, `{
			"items": {
				"`+itm.ID().Hex()+`": "deleted"
			}
		}`, string(bytes))

		watcher.Close()
	})
}

func TestWatcherSSE(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		watcher := NewWatcher(panicReporter)
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

		ctx, cancel := context.WithCancel(context.Background())

		rec := httptest.NewRecorder()
		data := base64.StdEncoding.EncodeToString([]byte(`{ "items": { "state": true } }`))
		req := httptest.NewRequest("GET", "/watch?s=items&d="+data, nil)
		req = req.WithContext(ctx)

		itm := &itemModel{
			Base: coal.B(),
			Bar:  "bar",
		}

		go func() {
			time.Sleep(100 * time.Millisecond)

			tester.Insert(itm)

			itm.Foo = "bar"
			tester.Replace(itm)

			tester.Delete(itm)

			time.Sleep(100 * time.Millisecond)

			cancel()
		}()

		group.Endpoint("").ServeHTTP(rec, req)

		assert.Equal(t, 200, rec.Code)
		assert.Equal(t, true, rec.Flushed)
		assert.Equal(t, []string{
			`data: {"items":{"` + itm.ID().Hex() + `":"created"}}`,
			`data: {"items":{"` + itm.ID().Hex() + `":"updated"}}`,
			`data: {"items":{"` + itm.ID().Hex() + `":"deleted"}}`,
		}, strings.Split(strings.TrimSpace(rec.Body.String()), "\n\n"))

		watcher.Close()
	})
}
