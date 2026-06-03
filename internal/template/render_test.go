package template_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cipta/crm-for-aiagents/internal/template"
)

func TestRenderSubstitutes(t *testing.T) {
	got, err := template.Render("Hi {{.FirstName}}", map[string]any{"FirstName": "Sam"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Hi Sam"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderMissingKeyNoError(t *testing.T) {
	got, err := template.Render("Hi {{.Name}}", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "<no value>") {
		t.Errorf("expected missing key to render empty, got %q", got)
	}
	want := "Hi "
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderInvalidTemplate(t *testing.T) {
	_, err := template.Render("{{.", nil)
	if err == nil {
		t.Fatal("expected error for invalid template, got nil")
	}
}

func TestRenderEmail(t *testing.T) {
	vars := map[string]any{"FirstName": "Sam"}
	res, err := template.RenderEmail("Subject: {{.FirstName}}", "HTML: {{.FirstName}}", "Text: {{.FirstName}}", vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Subject != "Subject: Sam" {
		t.Errorf("Subject got %q", res.Subject)
	}
	if res.HTML != "HTML: Sam" {
		t.Errorf("HTML got %q", res.HTML)
	}
	if res.Text != "Text: Sam" {
		t.Errorf("Text got %q", res.Text)
	}
}

func TestRewriteLinksReplacesAbsolute(t *testing.T) {
	html := `<a href="https://example.com/x">go</a>`
	makeCode := func(target string) (string, error) {
		if target != "https://example.com/x" {
			return "", errors.New("unexpected target")
		}
		return "abc123def456", nil
	}
	got, err := template.RewriteLinks(html, "https://crm.test", makeCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `<a href="https://crm.test/t/abc123def456">go</a>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewriteLinksLeavesRelativeAndMailto(t *testing.T) {
	html := `<a href="#top">top</a><a href="mailto:a@b">mail</a><a href="/rel">rel</a>`
	makeCode := func(target string) (string, error) {
		return "", errors.New("makeCode should not be called")
	}
	got, err := template.RewriteLinks(html, "https://crm.test", makeCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != html {
		t.Errorf("got %q, want %q", got, html)
	}
}

func TestRewriteLinksMakeCodeError(t *testing.T) {
	html := `<a href="https://example.com">go</a>`
	makeCode := func(target string) (string, error) {
		return "", errors.New("some error")
	}
	_, err := template.RewriteLinks(html, "https://crm.test", makeCode)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRewriteLinksMultiple(t *testing.T) {
	html := `<a href="https://example.com/1">one</a> and <a href="https://example.com/2">two</a>`
	codes := map[string]string{
		"https://example.com/1": "code1",
		"https://example.com/2": "code2",
	}
	makeCode := func(target string) (string, error) {
		return codes[target], nil
	}
	got, err := template.RewriteLinks(html, "https://crm.test", makeCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `<a href="https://crm.test/t/code1">one</a> and <a href="https://crm.test/t/code2">two</a>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInjectPixelBeforeBody(t *testing.T) {
	html := "<html><body>hi</body></html>"
	got := template.InjectPixel(html, "https://crm.test", "pixelcode")
	want := `<html><body>hi<img src="https://crm.test/o/pixelcode.png" width="1" height="1" alt="" style="display:none" /></body></html>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInjectPixelNoBody(t *testing.T) {
	html := "<p>hi</p>"
	got := template.InjectPixel(html, "https://crm.test", "pixelcode")
	want := `<p>hi</p><img src="https://crm.test/o/pixelcode.png" width="1" height="1" alt="" style="display:none" />`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInjectPixelURL(t *testing.T) {
	html := "test"
	got := template.InjectPixel(html, "https://crm.test", "abc")
	if !strings.Contains(got, `src="https://crm.test/o/abc.png"`) {
		t.Errorf("pixel URL not found or incorrect in %q", got)
	}
}

func TestRewriteLinksPreservesScriptAndStyle(t *testing.T) {
	html := `<html><head><style>body { color: red; }</style><script>if (a && b) { console.log("<test>"); }</script></head><body><a href="https://google.com">Google</a></body></html>`
	makeCode := func(target string) (string, error) {
		return "google_code", nil
	}
	got, err := template.RewriteLinks(html, "https://crm.test", makeCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, `if (a && b)`) {
		t.Errorf("script tag contents were altered or escaped: %s", got)
	}
	if !strings.Contains(got, `console.log("<test>")`) {
		t.Errorf("script tag inner content was altered or escaped: %s", got)
	}
	if !strings.Contains(got, `href="https://crm.test/t/google_code"`) {
		t.Errorf("link was not rewritten correctly: %s", got)
	}
}

func TestRewriteLinksCaseInsensitiveAndWhitespace(t *testing.T) {
	html := `<A   HRef="https://foo.com"  target="_blank" >foo</A>`
	makeCode := func(target string) (string, error) {
		return "foocode", nil
	}
	got, err := template.RewriteLinks(html, "https://crm.test", makeCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Note: since standard x/net/html parses and token.String() generates structured HTML,
	// the lowercase tag "a" and attributes may be reformatted, but other attributes are preserved.
	if !strings.Contains(got, `href="https://crm.test/t/foocode"`) {
		t.Errorf("failed to rewrite link with custom spacing/case: %s", got)
	}
	if !strings.Contains(got, `target="_blank"`) {
		t.Errorf("failed to preserve other attributes: %s", got)
	}
}

func TestRenderLiteralNoValuePreserved(t *testing.T) {
	got, err := template.Render("status: <no value> done", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "status: <no value> done"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderTopLevelMissingEmpty(t *testing.T) {
	got, err := template.Render("Hi {{.Name}}!", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Hi !"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewriteLinksUppercaseScheme(t *testing.T) {
	html := `<a href="HTTPS://example.com/x">go</a>`
	makeCode := func(target string) (string, error) {
		if target != "HTTPS://example.com/x" {
			return "", errors.New("unexpected target")
		}
		return "abc123def456", nil
	}
	got, err := template.RewriteLinks(html, "https://crm.test", makeCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `<a href="https://crm.test/t/abc123def456">go</a>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewriteLinksErrorWrapping(t *testing.T) {
	sentinelErr := errors.New("sentinel database error")
	html := `<a href="https://example.com/y">go</a>`
	makeCode := func(target string) (string, error) {
		return "", sentinelErr
	}
	_, err := template.RewriteLinks(html, "https://crm.test", makeCode)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinelErr) {
		t.Errorf("expected wrapped error to contain sentinelErr, got: %v", err)
	}
	expectedSub := `create tracking code for "https://example.com/y":`
	if !strings.Contains(err.Error(), expectedSub) {
		t.Errorf("expected error message to contain %q, got: %v", expectedSub, err)
	}
}
