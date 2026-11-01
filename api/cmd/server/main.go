package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cm-retailos-online/api/internal/config"
	"cm-retailos-online/api/internal/httpapi"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open PostgreSQL connection: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelStartup()
	if err := db.PingContext(startupCtx); err != nil {
		log.Fatalf("connect to PostgreSQL: %v", err)
	}

	api := httpapi.New(db, httpapi.Config{
		JWTSecret:          cfg.JWTSecret,
		CookieSecure:       cfg.CookieSecure,
		GoogleClientID:     cfg.GoogleClientID,
		GoogleClientSecret: cfg.GoogleClientSecret,
		GoogleRedirectURL:  cfg.GoogleRedirectURL,
		PublicLandingOnly:  cfg.PublicLandingOnly,
		PublicShopID:       cfg.PublicShopID,
		AppBaseURL:         cfg.AppBaseURL,
		SMTPHost:           cfg.SMTPHost,
		SMTPPort:           cfg.SMTPPort,
		SMTPUsername:       cfg.SMTPUsername,
		SMTPPassword:       cfg.SMTPPassword,
		SMTPFrom:           cfg.SMTPFrom,
	})
	if err := api.Migrate(startupCtx); err != nil {
		log.Fatalf("initialize PostgreSQL schema: %v", err)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		log.Printf("CM RetailOS Online API listening on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server: %v", err)
		}
	}()

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-stopCtx.Done()

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown: %v", err)
	}
}
