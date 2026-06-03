package mcpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMissingKey(t *testing.T) {
	innerCalled := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := AuthHandler("secret", inner)

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if innerCalled {
		t.Error("expected inner handler not to be called")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}
	expectedBody := `{"error":"unauthorized"}`
	if rr.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rr.Body.String())
	}
}

func TestAuthWrongKey(t *testing.T) {
	innerCalled := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := AuthHandler("secret", inner)

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer wrongsecret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if innerCalled {
		t.Error("expected inner handler not to be called")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestAuthValidBearer(t *testing.T) {
	innerCalled := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := AuthHandler("secret", inner)

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !innerCalled {
		t.Error("expected inner handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestAuthValidXAPIKey(t *testing.T) {
	innerCalled := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := AuthHandler("secret", inner)

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("X-API-Key", "secret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !innerCalled {
		t.Error("expected inner handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestAuthEmptyConfiguredKey(t *testing.T) {
	innerCalled := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := AuthHandler("", inner)

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if innerCalled {
		t.Error("expected inner handler not to be called")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestAuthBearerCaseInsensitive(t *testing.T) {
	innerCalled := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := AuthHandler("secret", inner)

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "bearer secret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !innerCalled {
		t.Error("expected inner handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
