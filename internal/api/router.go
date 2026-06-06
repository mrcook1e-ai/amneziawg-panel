package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
	"github.com/mrcook1e/amneziawg-panel/internal/static"
)

// spaHandler serves files from fsys and falls back to /index.html for any
// path that isn't an existing file (so Vue Router history mode works).
func spaHandler(fsys fs.FS) http.Handler {
	fileSrv := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(fsys, path); err == nil {
			fileSrv.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/index.html"
		fileSrv.ServeHTTP(w, r)
	})
}

func NewRouter(mgr *awg.Manager, auth *Auth, webFS http.FileSystem) http.Handler {
	h := &Handlers{Mgr: mgr, Auth: auth}

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/session", h.sessionGet)
		r.Post("/session", h.sessionPost)

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware)
			r.Delete("/session", h.sessionDelete)

			r.Route("/wireguard/client", func(r chi.Router) {
				r.Get("/", h.clientsList)
				r.Post("/", h.clientCreate)
				r.Delete("/{id}", h.clientDelete)
				r.Post("/{id}/enable", h.clientEnable)
				r.Post("/{id}/disable", h.clientDisable)
				r.Put("/{id}/name", h.clientRename)
				r.Put("/{id}/address", h.clientAddress)
				r.Get("/{id}/configuration", h.clientConfig)
				r.Get("/{id}/qrcode.svg", h.clientQR)
				r.Get("/{id}/amnezia.vpn", h.clientVPN)
				r.Get("/{id}/amnezia-qrcode.svg", h.clientVPNQR)
			})
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
