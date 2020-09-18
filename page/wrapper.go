package page

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ptt/pttweb/pttbbs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func setCommonResponseHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Server", "Cryophoenix")
	h.Set("Content-Type", "text/html; charset=utf-8")
}

type ErrorWrapper func(Context, http.ResponseWriter) error

func (fn ErrorWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCommonResponseHeaders(w)

	if err := clarifyRemoteError(handleRequest(w, r, fn)); err != nil {
		if pg, ok := err.(Page); ok {
			if err = ExecutePage(w, pg); err != nil {
				log.Println("Failed to emit error page:", err)
			}
			return
		}
		internalError(w, err)
	}
}

func clarifyRemoteError(err error) error {
	if err == pttbbs.ErrNotFound {
		return newNotFoundError(err)
	}

	switch grpc.Code(err) {
	case codes.NotFound, codes.PermissionDenied:
		return newNotFoundError(err)
	}

	return err
}

func internalError(w http.ResponseWriter, err error) {
	log.Println(err)
	w.WriteHeader(http.StatusInternalServerError)
	ExecutePage(w, &Error{
		Title:       `500 - Internal Server Error`,
		ContentHtml: `500 - Internal Server Error / Server Too Busy.`,
	})
}

func handleRequest(w http.ResponseWriter, r *http.Request, f func(Context, http.ResponseWriter) error) error {
	ctx, err := newContext(r)
	if err != nil {
		return err
	}
	return f(ctx, w)
}

type NotFoundError struct {
	NotFound
	UnderlyingErr error
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("not found error page: %v", e.UnderlyingErr)
}

func newNotFoundError(err error) *NotFoundError {
	return &NotFoundError{
		UnderlyingErr: err,
	}
}
