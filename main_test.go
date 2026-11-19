package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pauscout.local/app/reader"
)

func TestSaveCatalogEndpoints(t *testing.T) {
	path := os.Getenv("FM27_SAMPLE")
	if path == "" {
		t.Skip("FM27_SAMPLE non défini")
	}
	want, err := reader.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	type catalogSummary struct {
		Version              string                 `json:"version"`
		CompatibilityWarning string                 `json:"compatibility_warning"`
		GameDate             string                 `json:"game_date"`
		Coverage             reader.CatalogCoverage `json:"coverage"`
	}
	check := func(t *testing.T, summary catalogSummary) {
		t.Helper()
		if summary.Version != want.GameVersion || summary.CompatibilityWarning != want.CompatibilityWarning || summary.GameDate != want.GameDate || summary.Coverage.WithID == 0 {
			t.Fatalf("catalogue inattendu : %+v", summary)
		}
	}
	a := &api{}
	t.Run("import", func(t *testing.T) {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		request := httptest.NewRequest(http.MethodPost, "/api/charger", file)
		request.Header.Set("X-File-Name", url.QueryEscape(filepath.Base(path)))
		response := httptest.NewRecorder()
		a.upload(response, request)
		var summary catalogSummary
		if response.Code != http.StatusOK {
			t.Fatalf("import HTTP %d : %s", response.Code, response.Body)
		}
		if err := json.Unmarshal(response.Body.Bytes(), &summary); err != nil {
			t.Fatal(err)
		}
		check(t, summary)
	})
	t.Run("ouverture locale", func(t *testing.T) {
		body, err := json.Marshal(map[string]string{"path": path})
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		a.openLocal(response, httptest.NewRequest(http.MethodPost, "/api/ouvrir-local", bytes.NewReader(body)))
		if response.Code != http.StatusOK {
			t.Fatalf("ouverture locale HTTP %d : %s", response.Code, response.Body)
		}
		var result struct {
			Summary      catalogSummary `json:"summary"`
			Token        string         `json:"token"`
			WriteAllowed bool           `json:"write_allowed"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		check(t, result.Summary)
		wantWrite := reader.SupportsWriteVersion(want.GameVersion)
		if result.WriteAllowed != wantWrite || (result.Token != "") != wantWrite {
			t.Fatalf("capacité d'écriture incorrecte pour %s : %t", want.GameVersion, result.WriteAllowed)
		}
		if !wantWrite {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/enregistrer", bytes.NewBufferString(`{"token":"previous","edits":[]}`))
			request.Header.Set("Content-Type", "application/json")
			a.saveEdits(response, request)
			if response.Code != http.StatusConflict {
				t.Fatalf("tentative d'écriture en consultation seule : HTTP %d", response.Code)
			}
		}
	})
}

func TestTabLifecycleReloadAndLastTab(t *testing.T) {
	old := tabCloseDelay
	tabCloseDelay = 80 * time.Millisecond
	defer func() { tabCloseDelay = old }()
	a := &api{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/onglet", a.tabLifecycle)
	server := httptest.NewUnstartedServer(mux)
	a.srv = server.Config
	server.Start()
	defer server.Close()
	post := func(id, state string) {
		t.Helper()
		res, err := http.Post(server.URL+"/api/onglet?id="+id+"&state="+state, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("status=%d", res.StatusCode)
		}
	}
	post("premier-onglet", "open")
	post("premier-onglet", "close")
	post("nouvel-onglet", "open")
	time.Sleep(150 * time.Millisecond)
	post("nouvel-onglet", "open") // Reloading must cancel the pending shutdown.
	post("nouvel-onglet", "close")
	time.Sleep(150 * time.Millisecond)
	res, err := http.Post(server.URL+"/api/onglet?id=autre-onglet&state=open", "", nil)
	if err == nil {
		res.Body.Close()
		t.Fatal("le serveur aurait dû se fermer après le dernier onglet")
	}
}
