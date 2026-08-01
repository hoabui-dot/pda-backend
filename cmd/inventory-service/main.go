package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/company/pda-backend/internal/platform/httpserver"
)

func main() {
	if err := httpserver.Run("inventory-service", ":8083"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}
