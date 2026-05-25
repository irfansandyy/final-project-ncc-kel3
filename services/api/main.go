package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fpncc/api/config"
	"github.com/fpncc/api/db"
	"github.com/fpncc/api/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	cfg := config.Load()

	// Setup logging
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})).With("service", cfg.ServiceName)
	slog.SetDefault(logger)

	if cfg.PostgresDSN == "" {
		logger.Error("POSTGRES_DSN environment variable required")
		os.Exit(1)
	}

	database, err := db.Connect(cfg.PostgresDSN)
	if err != nil {
		logger.Error("failed to connect to db", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	logger.Info("connected to database")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// WebSocket Hub
	hub := handlers.NewHub(database, cfg.WSPollIntervalMs, logger)
	go hub.Run(ctx)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.AllowedOrigins},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Get("/ws", hub.HandleWebSocket())

	r.Route("/api/siem", func(r chi.Router) {
		// Overview (dashboard summary)
		r.Get("/overview", handlers.GetSiemOverview(database))

		// Events
		r.Get("/events", handlers.ListEvents(database))
		r.Get("/events/{id}", handlers.GetEvent(database))

		// Alerts
		r.Get("/alerts", handlers.ListAlerts(database))
		r.Patch("/alerts/{id}", handlers.UpdateAlertStatus(database))

		// Log Sources
		r.Get("/log-sources", handlers.ListLogSources(database))
		r.Post("/log-sources", handlers.CreateLogSource(database))
		r.Delete("/log-sources/{id}", handlers.DeleteLogSource(database))

		// Rules
		r.Get("/rules", handlers.ListRules(database))
		r.Get("/rules/{id}", handlers.GetRule(database))
		r.Post("/rules", handlers.CreateRule(database))
		r.Put("/rules/{id}", handlers.UpdateRule(database))
		r.Delete("/rules/{id}", handlers.DeleteRule(database))
		r.Post("/rules/reload", handlers.ReloadRules(database))
		r.Get("/rules/reload-status", handlers.ReloadStatus(database))
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		logger.Info("starting server", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", "error", err)
	}
	logger.Info("server stopped")
}
