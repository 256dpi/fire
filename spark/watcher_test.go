package spark

import (
	"net/http"
	"testing"
	"time"

	"github.com/256dpi/xo"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
)

func TestWatcher(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		watcher := NewWatcher(xo.Panic)
		watcher.Add(&Stream{
			Model: &itemModel{},
			Store: tester.Store,
		})

		group := tester.Assign("", &fire.Controller{
			Model: &itemModel{},
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
