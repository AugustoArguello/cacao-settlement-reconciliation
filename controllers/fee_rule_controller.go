package controllers

import (
	"net/http"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/middleware"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/repository"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/response"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type FeeRuleController struct {
	repo repository.FeeRuleRepository
}

func NewFeeRuleController(repo repository.FeeRuleRepository) *FeeRuleController {
	return &FeeRuleController{repo: repo}
}

func (ctrl *FeeRuleController) Create(c echo.Context) error {
	var rule models.FeeRule
	if err := c.Bind(&rule); err != nil {
		return middleware.NewBadRequestError("Failed to parse request body")
	}

	if rule.Processor == "" {
		return middleware.NewValidationError("processor is required")
	}
	if rule.RatePercent.IsNegative() {
		return middleware.NewValidationError("rate_percent must be non-negative")
	}

	rule.ID = uuid.New().String()

	if err := ctrl.repo.Store(c.Request().Context(), rule); err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, rule)
}

func (ctrl *FeeRuleController) List(c echo.Context) error {
	var processor *string
	if v := c.QueryParam("processor"); v != "" {
		processor = &v
	}

	rules, err := ctrl.repo.List(c.Request().Context(), processor)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, response.PaginatedResponse{
		Data:       rules,
		TotalCount: len(rules),
		Page:       1,
		PageSize:   len(rules),
	})
}
