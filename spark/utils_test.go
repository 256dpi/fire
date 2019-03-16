package spark

import (
	"net/http/httptest"
)

type responseRecorder struct {
	*httptest.ResponseRecorder
	closeNotify chan bool
}

func (m *responseRecorder) Close() {
	m.closeNotify <- true
}

func (m *responseRecorder) CloseNotify() <-chan bool {
	return m.closeNotify
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closeNotify: make(chan bool, 1),
	}
}
