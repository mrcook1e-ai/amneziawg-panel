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

	auth := api.NewAuth(cfg.Password)
	router := api.NewRouter(mgr, auth, nil) // TODO: embed.FS for web/

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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	_ = mgr.Shutdown()
}
