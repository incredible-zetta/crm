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

func TestClientListGroups(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/user/my/groups" {
			t.Fatalf("route = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"code":"SUCCESS","results":[{"jid":"120363@g.us","name":"Team","topic":"Ops","participants":[{},{}]}]}`))
	})
	defer srv.Close()
	groups, err := c.ListGroups(context.Background())
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups) != 1 || groups[0].JID != "120363@g.us" || groups[0].Participant != 2 {
		t.Fatalf("groups = %+v", groups)
	}
}

func TestClientSendImageByURL(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/send/image" {
			t.Fatalf("route = %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		if r.FormValue("phone") != "628123456789" {
			t.Errorf("phone = %q", r.FormValue("phone"))
		}
		if r.FormValue("caption") != "caption" || r.FormValue("image_url") != "https://example.com/x.jpg" {
			t.Errorf("form = %+v", r.MultipartForm.Value)
		}
		_, _ = w.Write([]byte(`{"code":"SUCCESS","message":"sent","results":{"message_id":"IMG1","status":"sent"}}`))
	})
	defer srv.Close()
	res, err := c.SendMedia(context.Background(), port.WhatsAppMediaMessage{Phone: "08123456789", Kind: "image", URL: "https://example.com/x.jpg", Caption: "caption"})
	if err != nil {
		t.Fatalf("SendMedia: %v", err)
	}
	if res.MessageID != "IMG1" {
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
