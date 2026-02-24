package middleware

import (
	"net/http"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/response"
	"github.com/labstack/echo/v4"
)

// DomainError represents a structured business error with an HTTP status code.
type DomainError struct {
	Code       string
	Message    string
	StatusCode int
	Field      string
}

func (e *DomainError) Error() string {
	return e.Message
}

// Common domain errors constructors

func NewValidationError(message string) *DomainError {
	return &DomainError{
		Code:       "VALIDATION_ERROR",
		Message:    message,
		StatusCode: http.StatusUnprocessableEntity,
	}
}

func NewDuplicateError(message string) *DomainError {
	return &DomainError{
		Code:       "DUPLICATE",
		Message:    message,
		StatusCode: http.StatusConflict,
	}
}

func NewNotFoundError(message string) *DomainError {
	return &DomainError{
		Code:       "NOT_FOUND",
		Message:    message,
		StatusCode: http.StatusNotFound,
	}
}

func NewBadRequestError(message string) *DomainError {
	return &DomainError{
		Code:       "INVALID_JSON",
		Message:    message,
		StatusCode: http.StatusBadRequest,
	}
}

// CustomHTTPErrorHandler provides consistent error response formatting across the API.
// It catches DomainErrors from services and translates them to structured JSON responses.
func CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	if domErr, ok := err.(*DomainError); ok {
		c.JSON(domErr.StatusCode, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    domErr.Code,
				Message: domErr.Message,
				Field:   domErr.Field,
			},
		})
		return
	}

	if echoErr, ok := err.(*echo.HTTPError); ok {
		code := echoErr.Code
		msg := "An unexpected error occurred"
		if m, ok := echoErr.Message.(string); ok {
			msg = m
		}
		c.JSON(code, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    http.StatusText(code),
				Message: msg,
			},
		})
		return
	}

	c.JSON(http.StatusInternalServerError, response.ErrorResponse{
		Error: response.ErrorDetail{
			Code:    "INTERNAL_ERROR",
			Message: "An unexpected error occurred",
		},
	})
}
