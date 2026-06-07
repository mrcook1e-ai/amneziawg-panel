package api

import (
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
	"github.com/mrcook1e/amneziawg-panel/internal/db"
	"github.com/mrcook1e/amneziawg-panel/internal/static"
)

// spaHandler serves embedded SPA assets, falling back to index.html for any
// path that doesn't match a real file (so client-side routing — /onboard/:token,
// /clients/:id etc — works on hard refresh / deep link).
//
// IMPORTANT: the fallback is served via direct byte-copy, NOT by reassigning
// r.URL.Path and calling http.FileServer. FileServer auto-redirects /foo/index.html
// to ./ (canonical form) which, from a deep URL like /onboard/<token>, resolves
// in the browser to /onboard/ and loops with any upstream slash-normalizing proxy.
func spaHandler(fsys fs.FS) http.Handler {
	fileSrv := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(fsys, path); err == nil {
				fileSrv.ServeHTTP(w, r)
				return
			}
		}
		f, err := fsys.Open("index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusNotFound)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = io.Copy(w, f)
	})
}

func NewRouter(mgr *awg.Manager, auth *Auth, stats *StatsHandlers, broker *Broker, webFS http.FileSystem) http.Handler {
	h := &Handlers{Mgr: mgr, Auth: auth}
	var adminDB *db.DB
	if stats != nil {
		adminDB = stats.DB
	}
	admin := &AdminHandlers{Mgr: mgr, DB: adminDB}

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", h.healthz)

	r.Route("/api", func(r chi.Router) {
		r.Get("/session", h.sessionGet)
		r.Post("/session", h.sessionPost)

		// Public cabinet — access token in URL is the authentication. The
		// subscriber sees their own devices and can add/remove without
		// admin involvement.
		r.Route("/cabinet/{token}", func(r chi.Router) {
			r.Get("/", h.cabinetGet)
			r.Post("/devices", h.cabinetAddDevice)
			r.Delete("/devices/{devId}", h.cabinetDeviceDelete)
			r.Get("/devices/{devId}/configuration", h.cabinetDeviceConfig)
			r.Get("/devices/{devId}/qrcode.svg", h.cabinetDeviceQR)
			r.Get("/devices/{devId}/amnezia.vpn", h.cabinetDeviceAmneziaVPN)
			r.Get("/devices/{devId}/amnezia-qrcode.svg", h.cabinetDeviceAmneziaQR)
			r.Get("/devices/{devId}/amnezia-qr-chunks", h.cabinetDeviceAmneziaQRChunks)
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware)
			r.Route("/subscribers", func(r chi.Router) {
				r.Get("/", h.subscribersList)
				r.Post("/", h.subscriberCreate)
				r.Get("/{id}", h.subscriberGet)
				r.Patch("/{id}", h.subscriberPatch)
				r.Delete("/{id}", h.subscriberDelete)
				r.Post("/{id}/regenerate-token", h.subscriberRegenToken)
				if stats != nil {
					r.Get("/{id}/stats", stats.subscriberStats)
				}
			})
			r.Delete("/session", h.sessionDelete)
			r.Get("/backup", admin.backup)
			r.Post("/restore", admin.restore)

			// Profiles — read-only list used by the frontend to populate
			// "Create client" and "Import client" dropdowns.
			r.Get("/profiles/", h.profilesList)

			r.Route("/wireguard/server", func(r chi.Router) {
				r.Post("/reset-clients", h.serverResetClients)
				r.Post("/factory-reset", admin.factoryReset)
			})

			r.Route("/wireguard/client", func(r chi.Router) {
				r.Get("/", h.clientsList)
				r.Post("/import", admin.importClient)
				r.Delete("/{id}", h.clientDelete)
				r.Post("/{id}/enable", h.clientEnable)
				r.Post("/{id}/disable", h.clientDisable)
				r.Put("/{id}/name", h.clientRename)
				r.Put("/{id}/address", h.clientAddress)
				r.Get("/{id}/configuration", h.clientConfig)
				r.Get("/{id}/qrcode.svg", h.clientQR)
				r.Get("/{id}/amnezia.vpn", h.clientAmneziaVPN)
				r.Get("/{id}/amnezia-qrcode.svg", h.clientAmneziaQR)
				r.Get("/{id}/amnezia-qr-chunks", h.clientAmneziaQRChunks)
				if stats != nil {
					r.Patch("/{id}", stats.clientPatch)
					r.Get("/{id}/stats", stats.clientStats)
					r.Get("/{id}/events", stats.clientEvents)
				}
			})

			if stats != nil {
				r.Route("/stats", func(r chi.Router) {
					r.Get("/overview", stats.overview)
					r.Get("/series", stats.series)
				})
				r.Get("/events", stats.eventsTail)
			}

			if broker != nil {
				r.Get("/stream", broker.stream)
			}
		})
	})

	switch {
	case webFS != nil:
		r.Handle("/*", http.FileServer(webFS))
	case static.Embedded && static.FS != nil:
		r.Handle("/*", spaHandler(static.FS))
	}
	return r
}
