package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPI_Root(t *testing.T) {
	mux := newTestServer(setupTestWorkspace(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for root, got %d", w.Result().StatusCode)
	}
}

func TestAPI_RootNotFound(t *testing.T) {
	mux := newTestServer(setupTestWorkspace(t))
	req := httptest.NewRequest(http.MethodGet, "/random-path", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 for unknown path, got %d", w.Result().StatusCode)
	}
}

func TestAPI_MethodNotAllowed(t *testing.T) {
	mux := newTestServer(setupTestWorkspace(t))

	reqs := []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/preview", nil),
		httptest.NewRequest(http.MethodPut, "/api/apply", nil),
	}

	for _, req := range reqs {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("Expected 405 Method Not Allowed for %s %s, got %d", req.Method, req.URL.Path, w.Result().StatusCode)
		}
	}
}

func TestAPI_InvalidJSON(t *testing.T) {
	mux := newTestServer(setupTestWorkspace(t))

	reqs := []*http.Request{
		httptest.NewRequest(http.MethodPost, "/api/preview", strings.NewReader("{bad json")),
		httptest.NewRequest(http.MethodPost, "/api/apply", strings.NewReader("{bad json")),
	}

	for _, req := range reqs {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request for invalid JSON on %s, got %d", req.URL.Path, w.Result().StatusCode)
		}
	}
}

func TestWithRecoveryAndCORS(t *testing.T) {
	handler := withRecoveryAndCORS(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	t.Run("OPTIONS Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK for OPTIONS, got %d", w.Result().StatusCode)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("Missing CORS headers")
		}
	})

	t.Run("Panic Recovery", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500 Internal Server Error, got %d", w.Result().StatusCode)
		}
		if !strings.Contains(w.Body.String(), "Server Panic: test panic") {
			t.Errorf("Expected panic message in JSON, got: %s", w.Body.String())
		}
	})
}
