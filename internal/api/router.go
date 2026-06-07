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

		// Public onboarding — invite link is the authentication. No middleware
		// applied so an unauthenticated browser can redeem and download.
		r.Get("/onboard/{token}", h.onboardStatus)
		r.Post("/onboard/{token}", h.onboardRedeem)

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware)
			r.Route("/onboard-tokens", func(r chi.Router) {
				r.Get("/", h.tokenList)
				r.Post("/", h.tokenCreate)
				r.Delete("/{id}", h.tokenRevoke)
			})
			r.Delete("/session", h.sessionDelete)
			r.Get("/backup", admin.backup)
			r.Post("/restore", admin.restore)

			r.Route("/profiles", func(r chi.Router) {
				r.Get("/", h.profilesList)
				r.Post("/", h.profileCreate)
				r.Get("/{id}", h.profileGet)
				r.Patch("/{id}", h.profilePatch)
				r.Delete("/{id}", h.profileDelete)
				r.Post("/{id}/restart", h.profileRestart)
			})

			r.Route("/wireguard/server", func(r chi.Router) {
				r.Post("/reset-clients", h.serverResetClients)
				r.Post("/factory-reset", admin.factoryReset)
			})

			r.Route("/wireguard/client", func(r chi.Router) {
				r.Get("/", h.clientsList)
				r.Post("/", h.clientCreate)
				r.Post("/import", admin.importClient)
				r.Delete("/{id}", h.clientDelete)
				r.Post("/{id}/enable", h.clientEnable)
				r.Post("/{id}/disable", h.clientDisable)
				r.Put("/{id}/name", h.clientRename)
				r.Put("/{id}/address", h.clientAddress)
				r.Patch("/{id}/profile", h.clientMove)
				r.Get("/{id}/configuration", h.clientConfig)
				r.Get("/{id}/qrcode.svg", h.clientQR)
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
