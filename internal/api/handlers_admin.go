package api

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
	"github.com/mrcook1e/amneziawg-panel/internal/db"
)

// AdminHandlers groups backup/restore + import + factory-reset. Kept in its
// own file so the core CRUD handlers stay readable.
type AdminHandlers struct {
	Mgr *awg.Manager
	DB  *db.DB
}

// backupFile defines what goes into the tar.gz. Paths inside the archive are
// kept flat so we don't leak the host's directory layout.
type backupFile struct {
	archiveName string
	diskPath    string
	optional    bool
}

func (a *AdminHandlers) files() []backupFile {
	dir := a.Mgr.StateDir()
	out := []backupFile{
		{archiveName: awg.StateFile, diskPath: filepath.Join(dir, awg.StateFile)},
		{archiveName: "panel.db", diskPath: filepath.Join(dir, "panel.db"), optional: true},
	}
	for _, iface := range a.Mgr.IfaceNames() {
		out = append(out, backupFile{
			archiveName: iface + ".conf",
			diskPath:    filepath.Join(dir, iface+".conf"),
			optional:    true,
		})
	}
	return out
}

func (a *AdminHandlers) backup(w http.ResponseWriter, r *http.Request) {
	fname := fmt.Sprintf("amneziawg-panel-%s.tar.gz", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+fname+`"`)

	gz := gzip.NewWriter(w)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	for _, f := range a.files() {
		info, err := os.Stat(f.diskPath)
		if err != nil {
			if os.IsNotExist(err) && f.optional {
				continue
			}
			http.Error(w, err.Error(), 500)
			return
		}
		body, err := os.ReadFile(f.diskPath)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		hdr := &tar.Header{
			Name:    f.archiveName,
			Size:    int64(len(body)),
			Mode:    0o600,
			ModTime: info.ModTime(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return
		}
		if _, err := tw.Write(body); err != nil {
			return
		}
	}
}

// restore accepts a multipart "file" or a raw tar.gz body. The archive is
// extracted into the state directory atomically (write to .restore-* then
// rename), then the manager is reloaded.
func (a *AdminHandlers) restore(w http.ResponseWriter, r *http.Request) {
	var src io.Reader = r.Body
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		if err := r.ParseMultipartForm(32 << 20); err == nil {
			if f, _, err := r.FormFile("file"); err == nil {
				defer f.Close()
				src = f
			}
		}
	}
	gz, err := gzip.NewReader(src)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "not a gzip archive: " + err.Error()})
		return
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	dir := a.Mgr.StateDir()
	allowed := map[string]bool{
		awg.StateFile: true,
		"panel.db":    true,
	}
	for _, iface := range a.Mgr.IfaceNames() {
		allowed[iface+".conf"] = true
	}

	var written []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "tar: " + err.Error()})
			return
		}
		base := filepath.Base(hdr.Name)
		if !allowed[base] {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(tr, 100<<20))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		dst := filepath.Join(dir, base)
		tmp := dst + ".restore-tmp"
		if err := os.WriteFile(tmp, body, 0o600); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		if err := os.Rename(tmp, dst); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		written = append(written, base)
	}

	if len(written) == 0 {
		writeJSON(w, 400, map[string]string{"error": "archive contained no recognized files"})
		return
	}
	if err := a.Mgr.Reload(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "reload: " + err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"success": true, "restored": written})
}

// factoryReset стирает все клиенты, перевыпускает ключ сервера и H1–H4,
// чистит метрики и журнал событий. На стороне клиента это эквивалент
// «удалить контейнер и развернуть заново», но без ручного редеплоя.
func (a *AdminHandlers) factoryReset(w http.ResponseWriter, r *http.Request) {
	if err := a.Mgr.FactoryReset(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "manager: " + err.Error()})
		return
	}
	if a.DB != nil {
		if err := a.DB.Reset(r.Context()); err != nil {
			writeJSON(w, 500, map[string]string{"error": "db: " + err.Error()})
			return
		}
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}

func (a *AdminHandlers) importClient(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name         string `json:"name"`
		SubscriberID string `json:"subscriberId"`
		ProfileID    string `json:"profileId"`
		PublicKey    string `json:"publicKey"`
		PrivateKey   string `json:"privateKey"`
		PreSharedKey string `json:"preSharedKey"`
		Address      string `json:"address"`
		Notes        string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Bad Request"})
		return
	}
	c, err := a.Mgr.ImportClient(awg.ImportArgs{
		Name: in.Name, SubscriberID: in.SubscriberID, ProfileID: in.ProfileID,
		PublicKey: in.PublicKey, PrivateKey: in.PrivateKey,
		PreSharedKey: in.PreSharedKey, Address: in.Address, Notes: in.Notes,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, c)
}
