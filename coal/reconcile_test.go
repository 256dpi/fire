package coal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReconcile(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		time.Sleep(10 * time.Millisecond)

		post := &postModel{
			Base:  B(),
			Title: "foo",
		}

		open := make(chan struct{})
		done := make(chan struct{})

		stream := Reconcile(tester.Store, &postModel{}, func() {
			close(open)
		}, func(model Model) {
			assert.Equal(t, post.ID(), model.ID())
			assert.Equal(t, "foo", model.(*postModel).Title)
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

		tester.Insert(post)

		post.Title = "bar"
		tester.Replace(post)

		tester.Delete(post)

		<-done

		stream.Close()
	})
}
