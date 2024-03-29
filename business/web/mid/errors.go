package mid

import (
	"context"
	"net/http"

	"github.com/piyush-saurabh/go-service/business/sys/validate"
	"github.com/piyush-saurabh/go-service/foundation/web"
	"go.uber.org/zap"
)

// Errors handles errors coming out of the call chain. It detects normal
// application errors which are used to respond to the client in a uniform way.
// Unexpected errors (status >= 500) are logged.
func Errors(log *zap.SugaredLogger) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(handler web.Handler) web.Handler {

		// Create the handler that will be attached in the middleware chain.
		// [PS] This is the handler layer outside the center layer (e.g Test)
		h := func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

			// [PS] since we are logging the error, we need the trace id from the context
			// If the context is missing this value, request the service
			// to be shutdown gracefully.
			v, err := web.GetValues(ctx)
			if err != nil {
				return web.NewShutdownError("web value missing from context")
			}

			// Run the next handler and catch any propagated error.
			// [PS] center of onion is 'Test' handler, outside it, we have Error handler
			if err := handler(ctx, w, r); err != nil {
				// Handle error coming from inner layer (e.g. Test)

				// Log the error.
				log.Errorw("ERROR", "traceid", v.TraceID, "ERROR", err)

				// [PS] know the type of error we received
				// Build out the error response.
				var er validate.ErrorResponse
				var status int
				switch act := validate.Cause(err).(type) {
				case validate.FieldErrors:
					er = validate.ErrorResponse{
						Error:  "data validation error",
						Fields: act.Error(),
					}
					status = http.StatusBadRequest

				case *validate.RequestError:
					er = validate.ErrorResponse{
						Error: act.Error(),
					}
					status = act.Status

				default:
					// untrusted error. Return 500
					er = validate.ErrorResponse{
						Error: http.StatusText(http.StatusInternalServerError),
					}
					status = http.StatusInternalServerError
				}

				// Respond with the error back to the client.
				if err := web.Respond(ctx, w, er, status); err != nil {
					return err
				}

				// If we receive the shutdown err we need to return it
				// back to the base handler to shutdown the service.
				if ok := web.IsShutdown(err); ok {
					return err
				}

			}

			// The error has been handled so we can stop propagating it.
			return nil //in best case
		}
		return h //returns handler
	}
	return m // returns middleware
}
