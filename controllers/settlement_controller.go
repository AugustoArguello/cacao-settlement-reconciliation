package controllers

import (
	"net/http"
	"time"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/middleware"
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
		return middleware.NewBadRequestError("Failed to parse request body")
	}

	report, err := ctrl.service.Create(c.Request().Context(), req)
	if err != nil {
		return err // DomainError handled by CustomHTTPErrorHandler
	}

	return c.JSON(http.StatusCreated, report)
}

func (ctrl *SettlementController) CreateBatch(c echo.Context) error {
	var req request.BatchSettlementRequest
	if err := c.Bind(&req); err != nil {
		return middleware.NewBadRequestError("Failed to parse request body")
	}

	if len(req.Settlements) == 0 {
		return middleware.NewBadRequestError("settlements array must not be empty")
	}

	ingested, duplicates, invalid, err := ctrl.service.CreateBatch(c.Request().Context(), req)
	if err != nil {
		return err
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
		return err
	}
	if report == nil {
		return middleware.NewNotFoundError("Settlement batch not found: " + batchID)
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
		return err
	}

	return c.JSON(http.StatusOK, response.PaginatedResponse{
		Data:       reports,
		TotalCount: total,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
	})
}
