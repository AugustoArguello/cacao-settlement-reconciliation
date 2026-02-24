package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/repository"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/request"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/response"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/services"
	"github.com/labstack/echo/v4"
)

type SettlementController struct {
	service *services.SettlementService
}

func NewSettlementController(service *services.SettlementService) *SettlementController {
	return &SettlementController{service: service}
}

func (ctrl *SettlementController) Create(c echo.Context) error {
	var req request.CreateSettlementRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INVALID_JSON",
				Message: "Failed to parse request body",
			},
		})
	}

	report, err := ctrl.service.Create(c.Request().Context(), req)
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
				Message: "Failed to create settlement",
			},
		})
	}

	return c.JSON(http.StatusCreated, report)
}

func (ctrl *SettlementController) CreateBatch(c echo.Context) error {
	var req request.BatchSettlementRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INVALID_JSON",
				Message: "Failed to parse request body",
			},
		})
	}

	if len(req.Settlements) == 0 {
		return c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "VALIDATION_ERROR",
				Message: "settlements array must not be empty",
			},
		})
	}

	ingested, duplicates, invalid, err := ctrl.service.CreateBatch(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to ingest settlements",
			},
		})
	}

	var errors []response.IngestionError
	for i, inv := range invalid {
		errors = append(errors, response.IngestionError{
			Index:   i,
			ID:      inv.BatchID,
			Message: "validation failed",
		})
	}

	return c.JSON(http.StatusCreated, response.BatchIngestionResponse{
		Ingested:          ingested,
		DuplicatesSkipped: duplicates,
		Errors:            errors,
	})
}

func (ctrl *SettlementController) GetByBatchID(c echo.Context) error {
	batchID := c.Param("id")
	report, err := ctrl.service.GetByBatchID(c.Request().Context(), batchID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to retrieve settlement",
			},
		})
	}
	if report == nil {
		return c.JSON(http.StatusNotFound, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "NOT_FOUND",
				Message: "Settlement batch not found: " + batchID,
			},
		})
	}
	return c.JSON(http.StatusOK, report)
}

func (ctrl *SettlementController) List(c echo.Context) error {
	filter := repository.SettlementFilter{
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

	reports, total, err := ctrl.service.List(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to list settlements",
			},
		})
	}

	return c.JSON(http.StatusOK, response.PaginatedResponse{
		Data:       reports,
		TotalCount: total,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
	})
}
