package main

import (
	"archive/zip"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"pauscout.local/app/reader"
)

//go:embed web/index.html
var website embed.FS

const (
	maxUpload = 800 << 20
	maxSave   = 1400 << 20
)

var tabCloseDelay = 30 * time.Second

type api struct {
	srv      *http.Server
	tabsMu   sync.Mutex
	tabs     map[string]bool
	tabsGen  uint64
	mu       sync.Mutex
	source   string
	size     int64
	modified time.Time
	token    string
}

// tabLifecycle waits through reloads and only stops after the last tab leaves.
func (a *api) tabLifecycle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.json(w, 405, map[string]string{"error": "Méthode invalide"})
		return
	}
	id := r.URL.Query().Get("id")
	if len(id) < 10 || len(id) > 100 {
		a.json(w, 400, map[string]string{"error": "Onglet invalide"})
		return
	}
	state := r.URL.Query().Get("state")
	if state != "open" && state != "close" {
		a.json(w, 400, map[string]string{"error": "État invalide"})
		return
	}
	a.tabsMu.Lock()
	if a.tabs == nil {
		a.tabs = make(map[string]bool)
	}
	if state == "open" {
		a.tabs[id] = true
	} else {
		delete(a.tabs, id)
	}
	a.tabsGen++
	gen := a.tabsGen
	shouldStop := state == "close" && len(a.tabs) == 0
	a.tabsMu.Unlock()
	a.json(w, 200, map[string]bool{"ok": true})
	if shouldStop {
		go func() {
			time.Sleep(tabCloseDelay)
			// An in-progress save holds mu until replacement is complete.
			a.mu.Lock()
			defer a.mu.Unlock()
			a.tabsMu.Lock()
			defer a.tabsMu.Unlock()
			if a.tabsGen == gen && len(a.tabs) == 0 && a.srv != nil {
				_ = a.srv.Close()
			}
		}()
	}
}

func (a *api) localSaves(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.json(w, 405, map[string]string{"error": "Méthode invalide"})
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		a.json(w, 500, map[string]string{"error": err.Error()})
		return
	}
	dir := filepath.Join(home, "Documents", "Sports Interactive", "Football Manager 27", "games")
	entries, err := os.ReadDir(dir)
	if err != nil {
		a.json(w, 200, map[string]any{"saves": []string{}, "folder": dir})
		return
	}
	saves := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".fm") {
			saves = append(saves, filepath.Join(dir, entry.Name()))
		}
	}
	a.json(w, 200, map[string]any{"saves": saves, "folder": dir})
}

func (a *api) openLocal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.json(w, 405, map[string]string{"error": "Méthode invalide"})
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
		a.json(w, 400, map[string]string{"error": "Chemin invalide"})
		return
	}
	path, err := filepath.Abs(strings.TrimSpace(req.Path))
	if err != nil || !strings.EqualFold(filepath.Ext(path), ".fm") {
		a.json(w, 400, map[string]string{"error": "Sélectionne une sauvegarde .fm locale"})
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > maxSave {
		a.json(w, 422, map[string]string{"error": "Sauvegarde locale absente ou invalide"})
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := reader.Read(path)
	if err != nil {
		a.json(w, 422, map[string]string{"error": err.Error()})
		return
	}
	if s.GameVersion != "27.0.1+2503222" {
		a.json(w, 422, map[string]string{"error": "Version FM27 non validée pour l'écriture : " + s.GameVersion})
		return
	}
	catalog, err := reader.ReadPlayerCatalog(path)
	if err != nil {
		a.json(w, 422, map[string]string{"error": err.Error()})
		return
	}
	var secret [24]byte
	if _, err = rand.Read(secret[:]); err != nil {
		a.json(w, 500, map[string]string{"error": "Session impossible"})
		return
	}
	a.source = path
	a.size = info.Size()
	a.modified = info.ModTime()
	a.token = hex.EncodeToString(secret[:])
	s.File = filepath.Base(path)
	s.Players = catalog.Players
	s.Staff = catalog.Staff
	s.EuroRate = catalog.EuroRate
	s.Coverage = &catalog.Coverage
	s.AttributeOrder = reader.AttributeOrder()
	s.StaffAttributeOrder = reader.StaffAttributeOrder()
	s.StaffRoleOrder = reader.StaffRoleOrder()
	a.json(w, 200, map[string]any{"summary": s, "path": path, "token": a.token})
}

func (a *api) saveEdits(w http.ResponseWriter, r *http.Request)      { a.saveChanges(w, r, false) }
func (a *api) saveStaffEdits(w http.ResponseWriter, r *http.Request) { a.saveChanges(w, r, true) }

func (a *api) saveChanges(w http.ResponseWriter, r *http.Request, staff bool) {
	if r.Method != http.MethodPost {
		a.json(w, 405, map[string]string{"error": "Méthode invalide"})
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		a.json(w, 415, map[string]string{"error": "JSON requis"})
		return
	}
	var req struct {
		Token string          `json:"token"`
		Edits json.RawMessage `json:"edits"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 65536)).Decode(&req); err != nil {
		a.json(w, 400, map[string]string{"error": "Modifications invalides : " + err.Error()})
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.source == "" || req.Token == "" || req.Token != a.token {
		a.json(w, 409, map[string]string{"error": "Rouvre la sauvegarde locale avant d'enregistrer"})
		return
	}
	info, err := os.Stat(a.source)
	if err != nil || info.Size() != a.size || !info.ModTime().Equal(a.modified) {
		a.json(w, 409, map[string]string{"error": "La sauvegarde a changé sur le disque ; rouvre-la avant d'enregistrer"})
		return
	}
	var count int
	if staff {
		var edits []reader.StaffEdit
		if err = json.Unmarshal(req.Edits, &edits); err == nil {
			count, err = reader.WriteStaffEdits(a.source, edits)
		}
	} else {
		var edits []reader.PlayerEdit
		if err = json.Unmarshal(req.Edits, &edits); err == nil {
			count, err = reader.WritePlayerEdits(a.source, edits)
		}
	}
	if err != nil {
		a.json(w, 422, map[string]string{"error": "Écriture interrompue : " + err.Error()})
		return
	}
	info, err = os.Stat(a.source)
	if err != nil {
		a.json(w, 500, map[string]string{"error": "Sauvegarde écrite, mais fichier inaccessible : " + err.Error()})
		return
	}
	a.size = info.Size()
	a.modified = info.ModTime()
	a.json(w, 200, map[string]any{"changed": count, "path": a.source})
}

func (a *api) json(w http.ResponseWriter, status int, obj any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(obj)
}

func (a *api) upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		a.json(w, 405, map[string]string{"error": "Methode invalide"})
		return
	}
	a.mu.Lock()
	a.source = ""
	a.token = ""
	a.mu.Unlock()
	if r.ContentLength > maxUpload {
		a.json(w, 413, map[string]string{"error": "Fichier trop volumineux (800 Mo maximum)"})
		return
	}
	rawName := strings.TrimSpace(r.Header.Get("X-File-Name"))
	decodedName, decErr := url.QueryUnescape(rawName)
	if decErr == nil {
		rawName = decodedName
	}
	name, err := filepath.Base(rawName), error(nil)
	if name == "." || name == "" {
		name = "sauvegarde.fm"
	}
	lname := strings.ToLower(name)
	if !strings.HasSuffix(lname, ".fm") && !strings.HasSuffix(lname, ".zip") {
		a.json(w, 400, map[string]string{"error": "Choisis un fichier .fm ou .zip"})
		return
	}
	dir, err := os.MkdirTemp("", "pauscout-fm-*")
	if err != nil {
		a.json(w, 500, map[string]string{"error": "Impossible de creer le dossier temporaire"})
		return
	}
	defer os.RemoveAll(dir)
	upload := filepath.Join(dir, "upload")
	dest, err := os.Create(upload)
	if err != nil {
		a.json(w, 500, map[string]string{"error": "Impossible de copier la sauvegarde"})
		return
	}
	n, err := io.Copy(dest, io.LimitReader(r.Body, maxUpload+1))
	closeErr := dest.Close()
	if err != nil || closeErr != nil || n > maxUpload {
		a.json(w, 413, map[string]string{"error": "Copie incomplete ou fichier trop volumineux"})
		return
	}
	savePath := upload
	if strings.HasSuffix(lname, ".zip") {
		z, err := zip.OpenReader(upload)
		if err != nil {
			a.json(w, 422, map[string]string{"error": "Archive ZIP illisible"})
			return
		}
		defer z.Close()
		var candidate *zip.File
		for _, f := range z.File {
			if !f.FileInfo().IsDir() && strings.HasSuffix(strings.ToLower(f.Name), ".fm") {
				if candidate != nil {
					a.json(w, 422, map[string]string{"error": "Le ZIP doit contenir une seule sauvegarde .fm"})
					return
				}
				candidate = f
			}
		}
		if candidate == nil || candidate.UncompressedSize64 > maxSave {
			a.json(w, 422, map[string]string{"error": "Pas de .fm unique ou archive trop volumineuse"})
			return
		}
		input, e := candidate.Open()
		if e != nil {
			a.json(w, 422, map[string]string{"error": "Impossible d'ouvrir la partie ZIP"})
			return
		}
		defer input.Close()
		savePath = filepath.Join(dir, "partie.fm")
		output, e := os.Create(savePath)
		if e != nil {
			a.json(w, 500, map[string]string{"error": "Impossible de creer la copie FM"})
			return
		}
		nc, e := io.Copy(output, io.LimitReader(input, maxSave+1))
		ec := output.Close()
		if e != nil || ec != nil || nc > maxSave {
			a.json(w, 422, map[string]string{"error": "Sauvegarde decompressee invalide ou trop volumineuse"})
			return
		}
	}
	s, err := reader.Read(savePath)
	if err != nil {
		a.json(w, 422, map[string]string{"error": "Lecture FMF impossible : " + err.Error()})
		return
	}
	s.File = name
	if s.GameVersion == "27.0.1+2503222" {
		catalog, catalogErr := reader.ReadPlayerCatalog(savePath)
		if catalogErr != nil {
			a.json(w, 422, map[string]string{"error": "Joueurs FM27 illisibles : " + catalogErr.Error()})
			return
		}
		s.Players = catalog.Players
		s.Staff = catalog.Staff
		s.EuroRate = catalog.EuroRate
		s.Coverage = &catalog.Coverage
		s.AttributeOrder = reader.AttributeOrder()
		s.StaffAttributeOrder = reader.StaffAttributeOrder()
		s.StaffRoleOrder = reader.StaffRoleOrder()
		s.Note = fmt.Sprintf("Liste partielle vérifiée : %d fiches liées à un ID (%d hommes, %d femmes), dont %d noms et %d clubs retrouvés, sur %d entrées annoncées. Le fichier n'a pas été modifié.", catalog.Coverage.WithID, catalog.Coverage.Men, catalog.Coverage.Women, catalog.Coverage.WithName, catalog.Coverage.WithClub, catalog.Coverage.Indexed)
	}
	a.json(w, 200, s)
}

func (a *api) close(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		a.json(w, 405, map[string]string{"error": "Methode invalide"})
		return
	}
	a.json(w, 200, map[string]bool{"stopped": true})
	go func() { time.Sleep(150 * time.Millisecond); _ = a.srv.Close() }()
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func run() error {
	mux := http.NewServeMux()
	a := &api{}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		data, err := website.ReadFile("web/index.html")
		if err != nil {
			http.Error(w, "Interface introuvable", 500)
			return
		}
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		a.json(w, 200, map[string]any{"native": true, "fmsave": true, "ready": true, "version": "0.14", "reader": "FMF/Zstandard en Go", "player_db": "partial", "staff_db": "partial"})
	})
	mux.HandleFunc("/api/charger", a.upload)
	mux.HandleFunc("/api/sauvegardes", a.localSaves)
	mux.HandleFunc("/api/ouvrir-local", a.openLocal)
	mux.HandleFunc("/api/enregistrer", a.saveEdits)
	mux.HandleFunc("/api/enregistrer-staff", a.saveStaffEdits)
	mux.HandleFunc("/api/quit", a.close)
	mux.HandleFunc("/api/onglet", a.tabLifecycle)
	listener, err := net.Listen("tcp", "127.0.0.1:48727")
	if err != nil {
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)
	a.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("PauScout FM27 v0.14 : %s", url)
	if err := openBrowser(url); err != nil {
		log.Printf("Ouvre ce lien manuellement : %s (%v)", url, err)
	}
	err = a.srv.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Println("Erreur PauScout:", err)
		os.Exit(1)
	}
}
