package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

//go:embed load-data.html
var loadDataPage []byte

// Caddy reverse-proxies https://<clinic>/load-data* here, so staff reach this
// page over the clinic's own address instead of a raw port. The port is open on
// this computer's interfaces because the Caddy container has to dial back in
// through the docker gateway; every action behind it needs the admin password.
const loadDataAddr = ":8765"

func (a *App) startLoadDataServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /load-data", a.serveLoadDataPage)
	mux.HandleFunc("GET /load-data/", a.serveLoadDataPage)
	mux.HandleFunc("POST /load-data/verify", func(w http.ResponseWriter, r *http.Request) {
		a.loadDataAction(w, r, nil)
	})
	mux.HandleFunc("POST /load-data/run", func(w http.ResponseWriter, r *http.Request) {
		a.loadDataAction(w, r, func(password string) error {
			return a.withJob(func() error {
				if err := a.requireStableClinic(); err != nil {
					return err
				}
				e := a.engine()
				e.AdminPassword = password
				return e.LoadDemoData()
			})
		})
	})

	srv := &http.Server{
		Addr:              loadDataAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	a.loadDataSrv = srv
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logln("The demo-data page could not be served: " + err.Error())
		}
	}()
}

func (a *App) stopLoadDataServer() {
	if a.loadDataSrv != nil {
		_ = a.loadDataSrv.Close()
	}
}

func (a *App) serveLoadDataPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(loadDataPage)
}

// loadDataAction checks the admin password, then runs step if there is one. The
// password travels with every request, so there is no session and no CSRF to
// get wrong. Requiring JSON keeps a browser from posting here cross-origin
// without a preflight.
func (a *App) loadDataAction(w http.ResponseWriter, r *http.Request, step func(string) error) {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		writeLoadDataJSON(w, http.StatusUnsupportedMediaType, "Unexpected request.")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeLoadDataJSON(w, http.StatusBadRequest, "Malformed request.")
		return
	}
	if err := a.requireAdmin(body.Password); err != nil {
		writeLoadDataJSON(w, http.StatusUnauthorized, err.Error())
		return
	}
	if step != nil {
		if err := step(body.Password); err != nil {
			writeLoadDataJSON(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeLoadDataJSON(w, http.StatusOK, "")
}

func writeLoadDataJSON(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": status == http.StatusOK, "error": message})
}
