package coal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReconcile(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		time.Sleep(100 * time.Millisecond)

		post := Init(&postModel{
			Title: "foo",
		}).(*postModel)

		tester.Save(post)

		open := make(chan struct{})
		done := make(chan struct{})

		stream := Reconcile(tester.Store, &postModel{}, func(model Model) {
			assert.Equal(t, post.ID(), model.ID())
			assert.Equal(t, "foo", model.(*postModel).Title)
			close(open)
		}, func(model Model) {
			assert.Equal(t, post.ID(), model.ID())
			assert.Equal(t, "bar", model.(*postModel).Title)
		}, func(id ID) {
			assert.Equal(t, post.ID(), id)
			close(done)
		}, func(err error) {
			panic(err)
		})

		<-open

		post.Title = "bar"
		tester.Update(post)
		tester.Delete(post)

		<-done

		stream.Close()
	})
}
