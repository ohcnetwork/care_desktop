package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoadDataActionGatesOnAdminPassword(t *testing.T) {
	a := settingsApp(t)

	cases := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
		wantRun     bool
	}{
		{"correct password runs the step", "application/json",
			`{"password":"` + settingsPassword + `"}`, http.StatusOK, true},
		{"wrong password is refused", "application/json",
			`{"password":"WrongPassword123"}`, http.StatusUnauthorized, false},
		{"empty password is refused", "application/json",
			`{}`, http.StatusUnauthorized, false},
		{"form posts are refused before the body is read", "text/plain",
			`{"password":"` + settingsPassword + `"}`, http.StatusUnsupportedMediaType, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ran := false
			req := httptest.NewRequest(http.MethodPost, "/load-data/run", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			rec := httptest.NewRecorder()

			a.loadDataAction(rec, req, func(string) error {
				ran = true
				return nil
			})

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body)
			}
			if ran != tc.wantRun {
				t.Errorf("step ran = %v, want %v", ran, tc.wantRun)
			}
		})
	}
}
