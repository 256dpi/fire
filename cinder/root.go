package cinder

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/opentracing/opentracing-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RootHandler is a middleware that can be used to create the root trace span
// for an incoming HTTP request.
func RootHandler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// split url
			segments := strings.Split(r.URL.Path, "/")

			// replace ids
			for i, s := range segments {
				if _, err := primitive.ObjectIDFromHex(s); err == nil {
					segments[i] = ":id"
				}
			}

			// construct name
			path := strings.Join(segments, "/")
			name := fmt.Sprintf("%s %s", r.Method, path)

			// create span from request
			span, ctx := opentracing.StartSpanFromContext(r.Context(), name)
			span.SetTag("peer.address", r.RemoteAddr)
			span.SetTag("http.proto", r.Proto)
			span.SetTag("http.method", r.Method)
			span.SetTag("http.host", r.Host)
			span.LogKV("http.url", r.URL.String())
			span.LogKV("http.length", r.ContentLength)
			span.LogKV("http.header", r.Header)

			// ensure finish
			defer span.Finish()

			// call next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
