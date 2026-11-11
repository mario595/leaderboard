package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
)

// statusClientClosedRequest is nginx's 499 "Client Closed Request". net/http
// has no constant for it. It is never received by anyone: it only records,
// in the request log, that the client left before the response.
const statusClientClosedRequest = 499

// clientGone reports whether err comes from the client closing the request:
// err wraps context.Canceled and the request's own context is done. Both are
// needed: a context.Canceled from any other context, with the request still
// alive, is a bug on our side. context.DeadlineExceeded is never the client
// (request contexts carry no deadline), so it is a server problem.
func clientGone(r *http.Request, err error) bool {
	return errors.Is(err, context.Canceled) && r.Context().Err() != nil
}

// writeServerError answers a request whose store call failed with err. When
// the client is gone (see clientGone) it logs at Debug and writes only a 499
// status, no body. Otherwise it logs msg at Error and answers 500 with a
// generic JSON error; err stays in the log, never in the response. args are
// slog key/value pairs added to the log line on both paths.
func writeServerError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, msg string, err error, args ...any) {
	if clientGone(r, err) {
		logger.With(args...).Debug("request canceled by client", "path", r.URL.Path, "error", err)
		w.WriteHeader(statusClientClosedRequest)
		return
	}
	logger.Error(msg, append(args, "error", err)...)
	writeError(w, logger, http.StatusInternalServerError, "internal error")
}
