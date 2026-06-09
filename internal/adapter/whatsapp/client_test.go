package whatsapp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	c, err := New(Config{BaseURL: srv.URL, BasicAuth: "dGVzdA==", DeviceID: "cds"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, srv
}

func TestClientCheck(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-device-id") != "cds" {
			t.Errorf("missing device id header")
		}
		if r.Header.Get("Authorization") != "Basic dGVzdA==" {
			t.Errorf("missing basic auth")
		}
		if r.URL.Query().Get("phone") != "628123456789" {
			t.Errorf("phone not normalized: %q", r.URL.Query().Get("phone"))
		}
		_, _ = w.Write([]byte(`{"code":"SUCCESS","message":"ok","results":{"is_on_whatsapp":true}}`))
	})
	defer srv.Close()

	res, err := c.Check(context.Background(), "08123456789")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Status != domain.WhatsAppRegistered {
		t.Errorf("status = %q, want registered", res.Status)
	}
}

func TestClientCheckNotRegistered(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":"SUCCESS","results":{"is_on_whatsapp":false}}`))
	})
	defer srv.Close()
	res, _ := c.Check(context.Background(), "628000000000")
	if res.Status != domain.WhatsAppNotRegistered {
		t.Errorf("status = %q, want not_registered", res.Status)
	}
}

func TestClientSend(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.FormValue("phone") != "628123456789" {
			t.Errorf("phone = %q", r.FormValue("phone"))
		}
		if !strings.Contains(r.FormValue("message"), "hello") {
			t.Errorf("message = %q", r.FormValue("message"))
		}
		_, _ = w.Write([]byte(`{"code":"SUCCESS","message":"Message sent","results":{"message_id":"WAMID123","status":"sent"}}`))
	})
	defer srv.Close()

	res, err := c.Send(context.Background(), port.WhatsAppMessage{Phone: "08123456789", Body: "hello"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if res.MessageID != "WAMID123" {
		t.Errorf("message id = %q", res.MessageID)
	}
}

func TestClientErrorSanitized(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"ERROR","message":"bad device"}`))
	})
	defer srv.Close()
	_, err := c.Send(context.Background(), port.WhatsAppMessage{Phone: "628123456789", Body: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "dGVzdA==") {
		t.Errorf("error leaked auth: %v", err)
	}
}
