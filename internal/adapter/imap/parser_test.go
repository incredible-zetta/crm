package imap

import (
	"strings"
	"testing"
)

func TestParsePlainTextMessage(t *testing.T) {
	raw := "From: Indra <INDRA@Example.COM>\r\nTo: no-reply@zettacrm.com\r\nSubject: Re: Promo Website\r\nMessage-ID: <m1@example.com>\r\nDate: Fri, 05 Jun 2026 10:00:00 +0000\r\n\r\nSaya tertarik, bisa diskusi?"
	msg, err := ParseMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if msg.FromEmail != "indra@example.com" || msg.FromName != "Indra" || msg.ToEmail != "no-reply@zettacrm.com" {
		t.Fatalf("bad addresses: %+v", msg)
	}
	if msg.Subject != "Re: Promo Website" || msg.MessageID != "<m1@example.com>" {
		t.Fatalf("bad headers: %+v", msg)
	}
	if msg.TextBody != "Saya tertarik, bisa diskusi?" || msg.Snippet != "Saya tertarik, bisa diskusi?" {
		t.Fatalf("bad body/snippet: %+v", msg)
	}
}

func TestParseMultipartAlternative(t *testing.T) {
	raw := "From: Lead <lead@example.com>\r\nTo: no-reply@zettacrm.com\r\nSubject: Hello\r\nContent-Type: multipart/alternative; boundary=b\r\n\r\n--b\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nPlain body\r\n--b\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<p>HTML body</p>\r\n--b--"
	msg, err := ParseMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if msg.TextBody != "Plain body" || msg.HTMLBody != "<p>HTML body</p>" || msg.Snippet != "Plain body" {
		t.Fatalf("bad multipart parse: %+v", msg)
	}
}

func TestParseHTMLFallbackSnippetAndMissingHeaders(t *testing.T) {
	raw := "From: lead@example.com\r\nTo: no-reply@zettacrm.com\r\nSubject: HTML\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<p>Hello <b>there</b></p>"
	msg, err := ParseMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if msg.MessageID != "" || msg.ReceivedAt.IsZero() {
		t.Fatalf("expected missing message id and fallback received time: %+v", msg)
	}
	if msg.Snippet != "Hello there" {
		t.Fatalf("bad html fallback snippet: %q", msg.Snippet)
	}
	if !strings.Contains(msg.RawHeadersJSON, "Subject") {
		t.Fatalf("expected raw headers json, got %q", msg.RawHeadersJSON)
	}
}
