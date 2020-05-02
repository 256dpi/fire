package spark

import (
	"net/http"
	"testing"
	"time"

	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
)

func TestWatcher(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		watcher := NewWatcher(xo.Crash)
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

		conn, err := Connect("ws://0.0.0.0:1234/watch")
		assert.NoError(t, err)
		assert.NotNil(t, conn)

		defer conn.Close()

		/* subscribe */

		err = conn.Send(nil, &Command{
			Type: Subscribe,
			Name: "items",
		})
		assert.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		/* create model */

		itm := tester.Insert(&itemModel{
			Bar: "bar",
		}).(*itemModel)

		var cmd Command
		err = conn.Receive(nil, &cmd)
		assert.NoError(t, err)
		assert.Equal(t, Command{
			Type: Created,
			Name: "items",
			ID:   itm.ID().Hex(),
		}, cmd)

		/* update model */

		itm.Foo = "bar"
		tester.Replace(itm)

		err = conn.Receive(nil, &cmd)
		assert.NoError(t, err)
		assert.Equal(t, Command{
			Type: Updated,
			Name: "items",
			ID:   itm.ID().Hex(),
		}, cmd)

		/* delete model */

		tester.Delete(itm)

		err = conn.Receive(nil, &cmd)
		assert.NoError(t, err)
		assert.Equal(t, Command{
			Type: Deleted,
			Name: "items",
			ID:   itm.ID().Hex(),
		}, cmd)

		/* close */

		watcher.Close()

		err = conn.Receive(nil, &cmd)
		assert.Error(t, err)
	})
}
