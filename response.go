package fire

import "net/http"

// implements the `api2go.Responder` interface
type response struct {
	Code int
	Data interface{}
}

func (r response) Metadata() map[string]interface{} {
	return nil
}

func (r response) Result() interface{} {
	return r.Data
}

func (r response) StatusCode() int {
	if r.Code == 0 {
		// return OK by default
		return http.StatusOK
	}

	return r.Code
}
