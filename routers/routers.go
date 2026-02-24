package routers

import (
	"net/http"
	"time"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/controllers"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/repository/memory"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/services"
	"github.com/labstack/echo/v4"
)

func ConfigRouter(e *echo.Echo) {
	// Initialize repositories
	txnRepo := memory.NewTransactionRepo()
	stlRepo := memory.NewSettlementRepo()

	// Initialize services
	txnService := services.NewTransactionService(txnRepo)
	stlService := services.NewSettlementService(stlRepo)

	// Initialize controllers
	txnCtrl := controllers.NewTransactionController(txnService)
	stlCtrl := controllers.NewSettlementController(stlService)

	// Health check
	e.GET("/health", healthCheck)

	// API v1 routes
	api := e.Group("/api")
	v1 := api.Group("/v1")

	// Transaction endpoints
	txns := v1.Group("/transactions")
	txns.POST("", txnCtrl.Create)
	txns.POST("/batch", txnCtrl.CreateBatch)
	txns.GET("", txnCtrl.List)
	txns.GET("/:id", txnCtrl.GetByID)

	// Settlement endpoints
	stls := v1.Group("/settlements")
	stls.POST("", stlCtrl.Create)
	stls.POST("/batch", stlCtrl.CreateBatch)
	stls.GET("", stlCtrl.List)
	stls.GET("/:id", stlCtrl.GetByBatchID)
}

func healthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"service": "cacao-settlement-reconciliation",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}
