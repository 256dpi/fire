package components

import (
	"net/http"
	"net/http/httptest"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
)

func testRequest(e *echo.Echo, method, path string, callback func(*httptest.ResponseRecorder, engine.Request)) {
	r, err := http.NewRequest(method, path, nil)
	if err != nil {
		panic(err)
	}

	rec := httptest.NewRecorder()

	req := standard.NewRequest(r, nil)
	res := standard.NewResponse(rec, nil)

	e.ServeHTTP(req, res)

	callback(rec, req)
}
