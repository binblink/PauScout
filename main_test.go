package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
