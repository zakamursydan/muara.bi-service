package httputil

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

type BaseResponse struct {
	ReqID   string `json:"reqId"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Error   any    `json:"error,omitempty"`
}

func getReqID(r *http.Request) string {
	reqID := middleware.GetReqID(r.Context())
	if reqID == "" {
		reqID = r.Header.Get("X-Request-Id")
	}
	return reqID
}

func writeJSON(w http.ResponseWriter, statusCode int, response BaseResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

func WriteSuccessResponse(w http.ResponseWriter, r *http.Request, message string, data any) {
	writeJSON(w, http.StatusOK, BaseResponse{
		ReqID:   getReqID(r),
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

func WriteCreatedResponse(w http.ResponseWriter, r *http.Request, message string, data any) {
	writeJSON(w, http.StatusCreated, BaseResponse{
		ReqID:   getReqID(r),
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

func WriteBadRequestResponse(w http.ResponseWriter, r *http.Request, message string, err any) {
	writeJSON(w, http.StatusBadRequest, BaseResponse{
		ReqID:   getReqID(r),
		Status:  "bad_request",
		Message: message,
		Error:   err,
	})
}

func WriteUnauthorizedResponse(w http.ResponseWriter, r *http.Request, message string) {
	writeJSON(w, http.StatusUnauthorized, BaseResponse{
		ReqID:   getReqID(r),
		Status:  "unauthorized",
		Message: message,
	})
}

func WriteForbiddenResponse(w http.ResponseWriter, r *http.Request, message string) {
	writeJSON(w, http.StatusForbidden, BaseResponse{
		ReqID:   getReqID(r),
		Status:  "forbidden",
		Message: message,
	})
}

func WriteNotFoundResponse(w http.ResponseWriter, r *http.Request, message string) {
	writeJSON(w, http.StatusNotFound, BaseResponse{
		ReqID:   getReqID(r),
		Status:  "not_found",
		Message: message,
	})
}

func WriteConflictResponse(w http.ResponseWriter, r *http.Request, message string, err any) {
	writeJSON(w, http.StatusConflict, BaseResponse{
		ReqID:   getReqID(r),
		Status:  "conflict",
		Message: message,
		Error:   err,
	})
}

func WriteInternalServerErrorResponse(w http.ResponseWriter, r *http.Request, message string, err any) {
	writeJSON(w, http.StatusInternalServerError, BaseResponse{
		ReqID:   getReqID(r),
		Status:  "internal_error",
		Message: message,
		Error:   err,
	})
}
