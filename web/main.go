package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/server"
	"github.com/999mattia/SwissWaterTemps/internal/sources"
	"github.com/999mattia/SwissWaterTemps/internal/store"
	"github.com/999mattia/SwissWaterTemps/internal/ui"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe /healthz of the running server and exit")
	flag.Parse()

	port := env("PORT", "3000")
	if *healthcheck {
		os.Exit(probe(port))
	}

	interval, err := time.ParseDuration(env("REFRESH_INTERVAL", "10m"))
	if err != nil {
		slog.Error("invalid REFRESH_INTERVAL", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := sources.NewHTTPClient()
	st := store.New(
		sources.NewBAFU(client),
		sources.NewBoot24(client),
		sources.NewHikaWetter(client),
	)
	go st.Run(ctx, interval)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           server.New(st.Snapshot, ui.FS()),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("listening", "addr", srv.Addr, "refresh", interval)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// probe is used as the container HEALTHCHECK, since the runtime image has no shell or curl.
func probe(port string) int {
	client := http.Client{Timeout: 3 * time.Second}
	res, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/healthz", port))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "unhealthy:", res.Status)
		return 1
	}
	return 0
}
