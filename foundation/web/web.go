// Package web contains a small web framework extension.
package web

import (
	"context"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/dimfeld/httptreemux/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

// [PS] This handler function is used to overcome the limitation of httptreemux handler. This will be inner layer of onion
// A Handler is a type that handles an http request within our own little mini
// framework.
type Handler func(ctx context.Context, w http.ResponseWriter, r *http.Request) error

// [PS] This struct represents a web application
// App is the entrypoint into our application and what configures our context
// object for each of our http handlers. Feel free to add any configuration
// data/logic on this App struct.
type App struct {
	mux      *httptreemux.ContextMux
	otmux    http.Handler // [PS] open telemetry support. use open telemetry as Handler
	shutdown chan os.Signal
	mw       []Middleware
}

// NewApp creates an App value that handle a set of routes for the application.
// [PS] pass 0 to many middleware functions
func NewApp(shutdown chan os.Signal, mw ...Middleware) *App {
	// Create an OpenTelemetry HTTP Handler which wraps our router. This will start
	// the initial span and annotate it with information about the request/response.
	//
	// This is configured to use the W3C TraceContext standard to set the remote
	// parent if an client request includes the appropriate headers.
	// https://w3c.github.io/trace-context/

	mux := httptreemux.NewContextMux()

	return &App{
		mux:      mux,
		otmux:    otelhttp.NewHandler(mux, "request"), // [PS] open telemetry mux is not the outmost layer of the onion
		shutdown: shutdown,
		mw:       mw,
	}
}

// SignalShutdown is used to gracefully shutdown the app when an integrity
// issue is identified.
func (a *App) SignalShutdown() {
	a.shutdown <- syscall.SIGTERM
}

// ServeHTTP implements the http.Handler interface. It's the entry point for
// all http traffic and allows the opentelemetry mux to run first to handle
// tracing. The opentelemetry mux then calls the application mux to handle
// application traffic. This was setup on line 58 in the NewApp function.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.otmux.ServeHTTP(w, r) // [PS] force otmux to execute 1st
}

// [PS] This is the outer layer. Actual treemux handle will be inside it
// Handle sets a handler function for a given HTTP method and path pair
// to the application server mux.
// [PS] group can be used for versioning v1, v2
func (a *App) Handle(method string, group string, path string, handler Handler, mw ...Middleware) {

	// First wrap handler specific middleware around this handler.
	// [PS] This will INJECT the middleware code and return the new handler
	handler = wrapMiddleware(mw, handler)

	// Add the application's general middleware to the handler chain.
	handler = wrapMiddleware(a.mw, handler)

	// The function to execute for each request.
	h := func(w http.ResponseWriter, r *http.Request) {

		// PRE CODE PROCESSING (logic should be in business layer - middleware)
		// Logging Started

		// Pull the context from the request and
		// use it as a separate parameter.
		ctx := r.Context()

		// Capture the parent request span from the context.
		// [PS] from the span, we can get the trace ID using: span.SpanContext().TraceID().String()
		span := trace.SpanFromContext(ctx)

		// Set the context with the required values to
		// process the request.
		v := Values{
			TraceID: span.SpanContext().TraceID().String(), // Generated using OpenTelemetry. alternative google uuid: uuid.New().String()
			Now:     time.Now(),
		}
		ctx = context.WithValue(ctx, key, &v) // key is package level variable in context.go

		// Call the wrapped handler functions.
		// [PS] context is passed to upper layer layer of the onion
		if err := handler(ctx, w, r); err != nil {
			// [PS] if error is in inner layer, there is something wrong so shutdown
			a.SignalShutdown()
			return
		}

		// Logging Ended
		// POST CODE PROCESSING (logic should be in business layer - middleware)

	}

	// Setting up the group (versioning)
	finalPath := path
	if group != "" {
		finalPath = "/" + group + path
	}

	a.mux.Handle(method, finalPath, h)
}
