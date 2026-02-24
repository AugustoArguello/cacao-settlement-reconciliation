package main

import (
	"fmt"
	"log"
	"os"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/config"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/routers"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	cfg := config.Load()

	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	routers.ConfigRouter(e)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Starting Cacao Direct Settlement Reconciliation API on %s", addr)

	if err := e.Start(addr); err != nil {
		log.Printf("Server stopped: %v", err)
		os.Exit(1)
	}
}
