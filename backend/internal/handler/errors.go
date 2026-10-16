package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"muara.bi/core/pkg/customerror"
	"muara.bi/core/pkg/httputil"
)

// WriteServiceError maps a service-layer error onto its HTTP status. Domain
// errors carry their own code via pkg/customerror; anything else is treated as
// an unexpected internal failure.
func WriteServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *customerror.AppError
	if !errors.As(err, &appErr) {
		httputil.WriteInternalServerErrorResponse(w, r, err.Error(), nil)
		return
	}

	detail := ""
	if appErr.Err != nil {
		detail = appErr.Err.Error()
	}

	switch appErr.Code {
	case customerror.ErrBadRequest:
		httputil.WriteBadRequestResponse(w, r, appErr.Message, detail)
	case customerror.ErrUnauthorized:
		httputil.WriteUnauthorizedResponse(w, r, appErr.Message)
	case customerror.ErrForbidden:
		httputil.WriteForbiddenResponse(w, r, appErr.Message)
	case customerror.ErrNotFound:
		httputil.WriteNotFoundResponse(w, r, appErr.Message)
	case customerror.ErrConflict:
		httputil.WriteConflictResponse(w, r, appErr.Message, detail)
	default:
		httputil.WriteInternalServerErrorResponse(w, r, appErr.Message, detail)
	}
}

// DecodeJSON reads a request body into dst.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		httputil.WriteBadRequestResponse(w, r, "Invalid request payload", err.Error())
		return false
	}
	return true
}
