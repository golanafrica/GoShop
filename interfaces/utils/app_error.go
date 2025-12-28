package utils

import (
	"encoding/json"
	"net/http"
)

type AppError struct {
	Code    string
	Message string
	Status  int
}

func NewAppError(code string, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
	}
}

func (e *AppError) Error() string {
	return e.Message
}

func WriteAppError(w http.ResponseWriter, appErr *AppError) {
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(appErr.Status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    appErr.Code,
		"message": appErr.Message,
		"status":  appErr.Status,
	})
}
