package httpresponse

import (
	"encoding/json"
	"net/http"
)

// SuccessResponse is the standard success envelope
type SuccessResponse struct {
	Status  string `json:"status"`
	Data    any    `json:"data"`
	Message string `json:"message,omitempty"`
}

// ErrorResponse is the standard error envelope
type ErrorResponse struct {
	StatusCode int    `json:"statusCode"`
	Error      string `json:"error"`
	Message    string `json:"message"`
}

// Success writes a 200 JSON success response
func Success(w http.ResponseWriter, data any, message string) {
	write(w, http.StatusOK, SuccessResponse{
		Status:  "success",
		Data:    data,
		Message: message,
	})
}

// Failed writes a JSON error response with the given status code
func Failed(w http.ResponseWriter, statusCode int, err string, message string) {
	write(w, statusCode, ErrorResponse{
		StatusCode: statusCode,
		Error:      err,
		Message:    message,
	})
}

// FailedBadRequest writes a 400 JSON error response
func FailedBadRequest(w http.ResponseWriter, err string, message string) {
	Failed(w, http.StatusBadRequest, err, message)
}

// FailedUnauthorized writes a 401 JSON error response
func FailedUnauthorized(w http.ResponseWriter, err string, message string) {
	Failed(w, http.StatusUnauthorized, err, message)
}

// FailedInternalServerError writes a 500 JSON error response
func FailedInternalServerError(w http.ResponseWriter, err string, message string) {
	Failed(w, http.StatusInternalServerError, err, message)
}

func write(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}
