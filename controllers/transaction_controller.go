package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/repository"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/request"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/response"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/services"
	"github.com/labstack/echo/v4"
)

type TransactionController struct {
	service *services.TransactionService
}

func NewTransactionController(service *services.TransactionService) *TransactionController {
	return &TransactionController{service: service}
}

func (ctrl *TransactionController) Create(c echo.Context) error {
	var req request.CreateTransactionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INVALID_JSON",
				Message: "Failed to parse request body",
			},
		})
	}

	txn, err := ctrl.service.Create(c.Request().Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "validation error") {
			return c.JSON(http.StatusUnprocessableEntity, response.ErrorResponse{
				Error: response.ErrorDetail{
					Code:    "VALIDATION_ERROR",
					Message: err.Error(),
				},
			})
		}
		if strings.Contains(err.Error(), "already exists") {
			return c.JSON(http.StatusConflict, response.ErrorResponse{
				Error: response.ErrorDetail{
					Code:    "DUPLICATE",
					Message: err.Error(),
				},
			})
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to create transaction",
			},
		})
	}

	return c.JSON(http.StatusCreated, txn)
}

func (ctrl *TransactionController) CreateBatch(c echo.Context) error {
	var req request.BatchTransactionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INVALID_JSON",
				Message: "Failed to parse request body",
			},
		})
	}

	if len(req.Transactions) == 0 {
		return c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "VALIDATION_ERROR",
				Message: "transactions array must not be empty",
			},
		})
	}

	ingested, duplicates, invalid, err := ctrl.service.CreateBatch(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to ingest transactions",
			},
		})
	}

	var errors []response.IngestionError
	for i, inv := range invalid {
		errors = append(errors, response.IngestionError{
			Index:   i,
			ID:      inv.TransactionID,
			Message: "validation failed",
		})
	}

	return c.JSON(http.StatusCreated, response.BatchIngestionResponse{
		Ingested:          ingested,
		DuplicatesSkipped: duplicates,
		Errors:            errors,
	})
}

func (ctrl *TransactionController) GetByID(c echo.Context) error {
	id := c.Param("id")
	txn, err := ctrl.service.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to retrieve transaction",
			},
		})
	}
	if txn == nil {
		return c.JSON(http.StatusNotFound, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "NOT_FOUND",
				Message: "Transaction not found: " + id,
			},
		})
	}
	return c.JSON(http.StatusOK, txn)
}

func (ctrl *TransactionController) List(c echo.Context) error {
	filter := repository.TransactionFilter{
		Page:     getIntParam(c, "page", 1),
		PageSize: getIntParam(c, "page_size", 50),
	}

	if v := c.QueryParam("processor"); v != "" {
		filter.Processor = &v
	}
	if v := c.QueryParam("currency"); v != "" {
		currency := models.Currency(v)
		filter.Currency = &currency
	}
	if v := c.QueryParam("status"); v != "" {
		status := models.TransactionStatus(v)
		filter.Status = &status
	}
	if v := c.QueryParam("country"); v != "" {
		filter.Country = &v
	}
	if v := c.QueryParam("date_from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.DateFrom = &t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			filter.DateFrom = &t
		}
	}
	if v := c.QueryParam("date_to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.DateTo = &t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			endOfDay := t.Add(24*time.Hour - time.Nanosecond)
			filter.DateTo = &endOfDay
		}
	}

	txns, total, err := ctrl.service.List(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to list transactions",
			},
		})
	}

	return c.JSON(http.StatusOK, response.PaginatedResponse{
		Data:       txns,
		TotalCount: total,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
	})
}

func getIntParam(c echo.Context, name string, defaultVal int) int {
	v := c.QueryParam(name)
	if v == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(v)
	if err != nil || i < 1 {
		return defaultVal
	}
	return i
}
