// Package api provides shared REST API response envelope and error types.
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// Error codes for the API error envelope.
const (
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeForbidden           = "FORBIDDEN"
	CodeValidationError     = "VALIDATION_ERROR"
	CodeNotFound            = "NOT_FOUND"
	CodeConflict            = "CONFLICT"
	CodeRateLimitExceeded   = "RATE_LIMIT_EXCEEDED"
	CodeInternalError       = "INTERNAL_ERROR"
	CodeBadRequest          = "BAD_REQUEST"
	CodeUnprocessableEntity = "UNPROCESSABLE_ENTITY"
	CodeServiceUnavailable  = "SERVICE_UNAVAILABLE"
)

// SuccessEnvelope is the standard success response shape: { "data": ..., "meta": ... }.
type SuccessEnvelope struct {
	Data interface{} `json:"data"`
	Meta *Meta      `json:"meta,omitempty"`
}

// Meta holds optional response metadata (e.g. requestId, pagination).
type Meta struct {
	RequestID string `json:"requestId,omitempty"`
}

// ErrorDetail is a field-level validation error.
type ErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ErrorEnvelope is the standard error response shape: { "error": { "code", "message", "requestId", "details" } }.
type ErrorEnvelope struct {
	Error ErrorPayload `json:"error"`
}

// ErrorPayload is the inner error object.
type ErrorPayload struct {
	Code      string        `json:"code"`
	Message   string        `json:"message"`
	RequestID string        `json:"requestId,omitempty"`
	Details   []ErrorDetail `json:"details,omitempty"`
}

// APIError carries status code and error code for handlers.
type APIError struct {
	StatusCode int           `json:"-"`
	Code       string        `json:"code"`
	Message    string        `json:"message"`
	Details    []ErrorDetail `json:"details,omitempty"`
}

func (e *APIError) Error() string { return e.Message }

// RequestID returns the request ID from the request (X-Request-ID header) or a new UUID.
func RequestID(r *http.Request) string {
	if r == nil {
		return ""
	}
	id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if id != "" {
		return id
	}
	return uuid.New().String()
}

// RespondJSON writes a success response with envelope: { "data": data, "meta": meta }.
func RespondJSON(w http.ResponseWriter, r *http.Request, statusCode int, data interface{}, meta *Meta) {
	if meta == nil && r != nil {
		meta = &Meta{RequestID: RequestID(r)}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(SuccessEnvelope{Data: data, Meta: meta})
}

// RespondError writes the error envelope and sets status code. Uses X-Request-ID from r or generates one.
func RespondError(w http.ResponseWriter, r *http.Request, err *APIError) {
	requestID := ""
	if r != nil {
		requestID = RequestID(r)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.StatusCode)
	_ = json.NewEncoder(w).Encode(ErrorEnvelope{
		Error: ErrorPayload{
			Code:      err.Code,
			Message:   err.Message,
			RequestID: requestID,
			Details:   err.Details,
		},
	})
}

// RespondNoContent writes 204 with no body.
func RespondNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
