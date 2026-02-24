package controllers

import (
	"net/http"
	"strings"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/repository"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/request"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/response"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/services"
	"github.com/labstack/echo/v4"
)

type ReconciliationController struct {
	service *services.ReconciliationService
}

func NewReconciliationController(service *services.ReconciliationService) *ReconciliationController {
	return &ReconciliationController{service: service}
}

func (ctrl *ReconciliationController) RunReconciliation(c echo.Context) error {
	var req request.RunReconciliationRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INVALID_JSON",
				Message: "Failed to parse request body",
			},
		})
	}

	report, err := ctrl.service.RunReconciliation(c.Request().Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "validation error") {
			return c.JSON(http.StatusUnprocessableEntity, response.ErrorResponse{
				Error: response.ErrorDetail{
					Code:    "VALIDATION_ERROR",
					Message: err.Error(),
				},
			})
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to run reconciliation: " + err.Error(),
			},
		})
	}

	return c.JSON(http.StatusOK, report)
}

func (ctrl *ReconciliationController) GetReport(c echo.Context) error {
	id := c.Param("id")
	report, err := ctrl.service.GetReport(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to retrieve report",
			},
		})
	}
	if report == nil {
		return c.JSON(http.StatusNotFound, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "NOT_FOUND",
				Message: "Report not found: " + id,
			},
		})
	}
	return c.JSON(http.StatusOK, report)
}

func (ctrl *ReconciliationController) ListReports(c echo.Context) error {
	reports, err := ctrl.service.ListReports(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to list reports",
			},
		})
	}
	return c.JSON(http.StatusOK, reports)
}

func (ctrl *ReconciliationController) ListDiscrepancies(c echo.Context) error {
	filter := repository.DiscrepancyFilter{
		Page:     getIntParam(c, "page", 1),
		PageSize: getIntParam(c, "page_size", 50),
	}

	if v := c.QueryParam("type"); v != "" {
		dt := models.DiscrepancyType(v)
		filter.Type = &dt
	}
	if v := c.QueryParam("severity"); v != "" {
		sev := models.Severity(v)
		filter.Severity = &sev
	}
	if v := c.QueryParam("processor"); v != "" {
		filter.Processor = &v
	}

	discs, total, err := ctrl.service.ListDiscrepancies(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to list discrepancies",
			},
		})
	}

	return c.JSON(http.StatusOK, response.PaginatedResponse{
		Data:       discs,
		TotalCount: total,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
	})
}
