// Package checkgrp maintains the group of handlers for health checking (liveliness and rediness probe in k8s).
package testgrp

import (
	"context"
	"errors"
	"math/rand"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/piyush-saurabh/go-service/foundation/web"
	"go.uber.org/zap"
)

// Handlers manages the set of check enpoints.
type Handlers struct {
	Log *zap.SugaredLogger
	DB  *sqlx.DB
}

// Test handler for poc
// [PS] This is the inner layer of the onion. If we react till here it means we have passed through all the middleware.
// [PS] It will return the result to the foundation layer
// [PS] Handler function will receive the request, get all the data from the request, call business layer (e.g. data layer - core/store), perform any business logic before of after the call to business layer
func (h Handlers) Test(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	// Testing for error handling
	if n := rand.Intn(100); n%2 == 0 {
		return errors.New("untrusted error") // should never see this error. Should return 500
		//return validate.NewRequestError(errors.New("trusted error"), http.StatusBadRequest) // 400 error. output message will be send to browser
		//return web.NewShutdownError("restart service") // Shutdown error (foundation layer)
		//panic("testing panic") // Test the panic scenario
	}

	status := struct {
		Status string
	}{
		Status: "OK",
	}

	//statusCode := http.StatusOK

	// Pod level logging (in prod it will be handled via middleware in business layer)
	//h.Log.Infow("readiness", "statusCode", statusCode, "method", r.Method, "path", r.URL.Path, "remoteaddr", r.RemoteAddr)

	return web.Respond(ctx, w, status, http.StatusOK)
}
