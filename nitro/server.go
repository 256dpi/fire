package nitro

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/256dpi/serve"
	"github.com/256dpi/xo"
)

// Context allows handlers to interact with the request.
type Context struct {
	context.Context

	// The received procedure.
	Procedure Procedure

	// The current handler.
	Handler *Handler

	// The underlying request.
	Request *http.Request

	// The underlying writer.
	Writer http.ResponseWriter
}

// Handler defines a procedure handler.
type Handler struct {
	// The handled procedure.
	Procedure Procedure

	// The request body limit.
	//
	// Default: 4K.
	Limit int64

	// The processing callback.
	Callback func(ctx *Context) error
}

// Endpoint manages the calling of procedures.
type Endpoint struct {
	handlers map[string]*Handler
	reporter func(error)
}

// NewEndpoint creates and returns a new server.
func NewEndpoint(reporter func(error)) *Endpoint {
	return &Endpoint{
		handlers: map[string]*Handler{},
		reporter: reporter,
	}
}

// Add will add the specified handler to the endpoint.
func (s *Endpoint) Add(handler *Handler) {
	// set default limit
	if handler.Limit == 0 {
		handler.Limit = serve.MustByteSize("4K")
	}

	// get name
	name := GetMeta(handler.Procedure).Name

	// add handler
	s.handlers[name] = handler
}

// ServerHTTP implements the http.Handler interface.
func (s *Endpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// check request method
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// get clean url
	url := strings.Trim(r.URL.Path, "/")

	// lookup handler
	handler, ok := s.handlers[url]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// prepare context
	ctx := &Context{
		Context:   r.Context(),
		Procedure: GetMeta(handler.Procedure).Make(),
		Handler:   handler,
		Request:   r,
		Writer:    w,
	}

	// process context
	err := s.Process(ctx)
	if err != nil && s.reporter != nil {
		s.reporter(err)
	}
}

// Process handles the specified context.
func (s *Endpoint) Process(ctx *Context) error {
	// get meta
	meta := GetMeta(ctx.Procedure)

	// limit body
	serve.LimitBody(ctx.Writer, ctx.Request, ctx.Handler.Limit)

	// read body
	body, err := ioutil.ReadAll(ctx.Request.Body)
	if err != nil {
		return xo.W(err)
	}

	// unmarshal body
	err = meta.Coding.Unmarshal(body, ctx.Procedure)
	if err != nil {
		return err
	}

	// pre validate
	err = ctx.Procedure.Validate()
	if err != nil {
		return xo.W(err)
	}

	// run callback
	err = xo.Catch(func() error {
		return ctx.Handler.Callback(ctx)
	})
	if err != nil {
		// check if safe error
		if xo.IsSafe(err) {
			err = BadRequest(xo.AsSafe(err).Msg, "")
		}

		// check if rich error
		anError := AsError(err)
		if anError != nil {
			// unset original error
			err = nil
		} else {
			// create internal server error
			anError = ErrorFromStatus(http.StatusInternalServerError, "")
		}

		// set fallback status
		if http.StatusText(anError.Status) == "" {
			anError.Status = http.StatusInternalServerError
		}

		// write header
		ctx.Writer.Header().Set("Content-Type", mimeTypes[meta.Coding])
		ctx.Writer.WriteHeader(anError.Status)

		// write body
		body, _ := meta.Coding.Marshal(anError)
		_, _ = ctx.Writer.Write(body)

		return err
	}

	// post validate
	err = ctx.Procedure.Validate()
	if err != nil {
		return xo.W(err)
	}

	// write header
	ctx.Writer.Header().Set("Content-Type", mimeTypes[meta.Coding])
	ctx.Writer.WriteHeader(http.StatusOK)

	// write response
	body, _ = meta.Coding.Marshal(ctx.Procedure)
	_, _ = ctx.Writer.Write(body)

	return nil
}
