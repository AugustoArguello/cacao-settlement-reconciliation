package routers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

func ConfigRouter(e *echo.Echo) {
	e.GET("/health", healthCheck)

	api := e.Group("/api")
	v1 := api.Group("/v1")

	_ = v1 // Routes will be added as features are implemented
}

func healthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"service": "cacao-settlement-reconciliation",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}
