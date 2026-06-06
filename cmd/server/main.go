package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mrcook1e/amneziawg-panel/internal/api"
	"github.com/mrcook1e/amneziawg-panel/internal/awg"
	"github.com/mrcook1e/amneziawg-panel/internal/config"
	"github.com/mrcook1e/amneziawg-panel/internal/db"
	"github.com/mrcook1e/amneziawg-panel/internal/events"
	"github.com/mrcook1e/amneziawg-panel/internal/stats"
)

func main() {
	cfg := config.Load()

	mgr, err := awg.NewManager(cfg)
	if err != nil {
		log.Fatalf("manager init: %v", err)
	}
	if err := mgr.Start(); err != nil {
		log.Fatalf("manager start: %v", err)
	}

	// Metrics + events store. Lives next to the wg config so it gets backed
	// up by the same volume mount the rest of the state uses.
	d, err := db.Open(cfg.WGPath)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer d.Close()

	evLog := events.New(d)
	mgr.SetEventSink(evLog.Append)

	collector := &stats.Collector{
		DB: d, Mgr: mgr, Events: evLog,
		Tick: 30 * time.Second,
		Bin:  cfg.AWGBin,
	}
	collectorCtx, stopCollector := context.WithCancel(context.Background())
	go collector.Run(collectorCtx)

	auth := api.NewAuth(cfg.Password)
	sh := &api.StatsHandlers{Mgr: mgr, DB: d, Events: evLog}

	// SSE-брокер: 1с-тик с живой скоростью + push событий из журнала.
	broker := api.NewBroker(mgr, cfg.AWGBin)
	broker.AttachEventLog(evLog)
	brokerCtx, stopBroker := context.WithCancel(context.Background())
	go broker.Run(brokerCtx)

	router := api.NewRouter(mgr, auth, sh, broker, nil)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.HTTPPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	stopCollector()
	stopBroker()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	_ = mgr.Shutdown()
}
