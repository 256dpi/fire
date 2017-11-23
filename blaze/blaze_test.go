package blaze

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestJobController(t *testing.T) {
	tester.Clean()
	tester.Assign("", JobController(tester.Store))

	coll := C(tester.Store.Copy())

	id, err := coll.Enqueue("Test", bson.M{"foo": "bar"}, 0)
	assert.NoError(t, err)

	err = coll.Complete(id, bson.M{"bar": "foo"})
	assert.NoError(t, err)

	job, err := coll.Fetch(id)
	assert.NoError(t, err)

	tester.Request("GET", "/jobs", "", func(rr *httptest.ResponseRecorder, r *http.Request) {
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, `{
			"data":[
				{
					"type": "jobs",
					"id": "`+id.Hex()+`",
					"attributes": {
						"name": "Test",
						"params": {"foo":"bar"},
						"status": "completed",
						"created": "`+job.Created.Format(time.RFC3339Nano)+`",
						"attempts": 0,
						"started": "0001-01-01T00:00:00Z",
						"delayed":"`+job.Delayed.Format(time.RFC3339Nano)+`",
						"ended":"`+job.Ended.Format(time.RFC3339Nano)+`",
						"result": {"bar":"foo"}
					}
				}
			],
			"links": {
				"self": "/jobs"
			}
		}`, rr.Body.String())
	})
}
