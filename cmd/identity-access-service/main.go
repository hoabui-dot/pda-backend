package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/company/pda-backend/internal/platform/httpserver"
)

func main() {
	if err := httpserver.Run("identity-access-service", ":8081"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}
