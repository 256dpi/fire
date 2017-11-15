package flame

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/256dpi/fire/coal"

	"github.com/stretchr/testify/assert"
)

// Map is a general map.
type Map map[string]interface{}

// AssertResourceRepresentation will load the specified resource and check if its
// representation matches the supplied format.
func (t *Tester) AssertResourceRepresentation(tt assert.TestingT, model coal.Model, token string, attributes Map) {
	resource := model.Meta().PluralName
	id := model.ID().Hex()

	// encode attributes
	object, err := json.Marshal(attributes)
	assert.NoError(tt, err)

	t.Request("GET", resource+"/"+id, map[string]string{
		"Authorization": "Bearer " + token,
	}, "", func(rr *httptest.ResponseRecorder, r *http.Request) {
		assert.Equal(tt, http.StatusOK, rr.Result().StatusCode)
		assert.JSONEq(tt, `{
			"data": {
				"type": "`+resource+`",
				"id": "`+id+`",
				"attributes": `+string(object)+`
			},
			"links": {
				"self": "/`+t.Prefix+"/"+resource+"/"+id+`"
			}
		}`, rr.Body.String(), debugRequest(r, rr))

		/*
			"relationships": {
				"comments": {
					"data": [],
					"links": {
						"self": "/posts/`+id+`/relationships/comments",
						"related": "/posts/`+id+`/comments"
					}
				},
				"selections": {
					"data": [],
					"links": {
						"self": "/posts/`+id+`/relationships/selections",
						"related": "/posts/`+id+`/selections"
					}
				}
			}
		*/
	})
}
