package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mrcook1e/amneziawg-panel/internal/api"
	"github.com/mrcook1e/amneziawg-panel/internal/awg"
	"github.com/mrcook1e/amneziawg-panel/internal/billing"
	"github.com/mrcook1e/amneziawg-panel/internal/config"
	"github.com/mrcook1e/amneziawg-panel/internal/db"
	"github.com/mrcook1e/amneziawg-panel/internal/events"
	"github.com/mrcook1e/amneziawg-panel/internal/logging"
	"github.com/mrcook1e/amneziawg-panel/internal/stats"
)

func main() {
	logger, err := logging.New(os.Stdout, logging.Config{
		Format: os.Getenv("LOG_FORMAT"),
		Level:  os.Getenv("LOG_LEVEL"),
	})
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stdout, nil)).Error("logging configuration invalid",
			slog.String("component", "logging"),
			slog.Any("error", err),
		)
		os.Exit(1)
	}
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		exitStartup("config", "network configuration invalid", err)
	}
	slog.Info("config loaded",
		slog.String("component", "config"),
		slog.String("wg_host", cfg.WGHost),
		slog.Int("port_range_start", cfg.PortRangeStart),
		slog.Int("port_range_end", cfg.PortRangeEnd),
		slog.String("state_path", cfg.WGPath),
	)
	slog.Info("egress resolved",
		slog.String("component", "network"),
		slog.String("egress", cfg.EgressIface),
	)

	mgr, err := awg.NewManager(cfg)
	if err != nil {
		exitStartup("manager", "manager initialization failed", err)
	}
	if err := mgr.Start(); err != nil {
		exitStartup("manager", "manager startup failed", err)
	}
	slog.Info("manager ready", slog.String("component", "manager"))

	// Metrics + events store. Lives next to the wg config so it gets backed
	// up by the same volume mount the rest of the state uses.
	d, err := db.Open(cfg.WGPath)
	if err != nil {
		exitStartup("database", "database open failed", err)
	}
	slog.Info("database ready",
		slog.String("component", "database"),
		slog.String("state_path", cfg.WGPath),
	)

	evLog := events.New(d)
	mgr.SetEventSink(evLog.Append)

	collector := &stats.Collector{
		DB: d, Mgr: mgr,
		Tick: 30 * time.Second,
		Bin:  cfg.AWGBin,
	}
	collectorCtx, stopCollector := context.WithCancel(context.Background())
	go collector.Run(collectorCtx)

	auth := api.NewAuth(cfg.Password)
	sh := &api.StatsHandlers{Mgr: mgr, DB: d, Events: evLog}
	billingSvc := billing.NewService(d, mgr, cfg)
	billingSvc.StartBackgroundLoop()

	// SSE-брокер: 1с-тик с живой скоростью + push событий из журнала.
	broker := api.NewBroker(mgr, cfg.AWGBin)
	broker.AttachEventLog(evLog)
	brokerCtx, stopBroker := context.WithCancel(context.Background())
	go broker.Run(brokerCtx)
	slog.Info("background services ready", slog.String("component", "background"))

	router := api.NewRouter(mgr, auth, sh, broker, billingSvc, nil)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.HTTPPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	serverErr := make(chan error, 1)
	slog.Info("HTTP listen",
		slog.String("component", "http"),
		slog.String("listen_address", srv.Addr),
	)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)

	serverFailed := false
	select {
	case received := <-stop:
		slog.Info("signal received",
			slog.String("component", "lifecycle"),
			slog.String("signal", received.String()),
		)
	case err := <-serverErr:
		slog.Error("HTTP server failed",
			slog.String("component", "http"),
			slog.Any("error", err),
		)
		serverFailed = true
	}

	stopCollector()
	stopBroker()
	billingSvc.StopBackgroundLoop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	shutdownFailed := false
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("HTTP shutdown failed",
			slog.String("component", "http"),
			slog.Any("error", err),
		)
		shutdownFailed = true
	}
	if err := mgr.Shutdown(); err != nil {
		slog.Error("manager shutdown failed",
			slog.String("component", "manager"),
			slog.Any("error", err),
		)
		shutdownFailed = true
	}
	if err := d.Close(); err != nil {
		slog.Error("database shutdown failed",
			slog.String("component", "database"),
			slog.Any("error", err),
		)
		shutdownFailed = true
	}
	if serverFailed || shutdownFailed {
		os.Exit(1)
	}
	slog.Info("service stopped", slog.String("component", "lifecycle"))
}

func exitStartup(component, message string, err error) {
	slog.LogAttrs(context.Background(), slog.LevelError, message, startupErrorAttrs(component, err)...)
	os.Exit(1)
}

func startupErrorAttrs(component string, err error) []slog.Attr {
	attrs := []slog.Attr{slog.String("component", component)}
	var environmentError *config.EnvironmentError
	if errors.As(err, &environmentError) {
		return append(attrs,
			slog.String("field", environmentError.Field),
			slog.String("rule", environmentError.Rule),
		)
	}
	return append(attrs, slog.Any("error", err))
}
