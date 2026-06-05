package template

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	texttemplate "text/template"
	"text/template/parse"

	xhtml "golang.org/x/net/html"
)

type Rendered struct {
	Subject, HTML, Text string
}

// Render parses the template and executes it with the provided variables.
// Supports flat top-level variables ({{.Key}}). Missing top-level keys render as empty.
var bareMergeVarPattern = regexp.MustCompile(`{{\s*([A-Za-z_][A-Za-z0-9_]*)\s*}}`)

func Render(tmpl string, vars map[string]any) (string, error) {
	tmpl = normalizeBareMergeVars(tmpl)
	t, err := texttemplate.New("tmpl").Option("missingkey=zero").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	varsCopy := make(map[string]any)
	for k, v := range vars {
		varsCopy[k] = v
	}

	prePopulateMissingKeys(t.Tree.Root, varsCopy)

	var buf strings.Builder
	if err := t.Execute(&buf, varsCopy); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func normalizeBareMergeVars(tmpl string) string {
	return bareMergeVarPattern.ReplaceAllString(tmpl, `{{.$1}}`)
}

func prePopulateMissingKeys(node parse.Node, vars map[string]any) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *parse.ListNode:
		for _, child := range n.Nodes {
			prePopulateMissingKeys(child, vars)
		}
	case *parse.ActionNode:
		prePopulateMissingKeys(n.Pipe, vars)
	case *parse.PipeNode:
		for _, cmd := range n.Cmds {
			prePopulateMissingKeys(cmd, vars)
		}
	case *parse.CommandNode:
		for _, arg := range n.Args {
			prePopulateMissingKeys(arg, vars)
		}
	case *parse.FieldNode:
		if len(n.Ident) == 1 {
			key := n.Ident[0]
			if _, exists := vars[key]; !exists {
				vars[key] = ""
			}
		}
	case *parse.IfNode:
		prePopulateMissingKeys(n.Pipe, vars)
		prePopulateMissingKeys(n.List, vars)
		prePopulateMissingKeys(n.ElseList, vars)
	case *parse.RangeNode:
		prePopulateMissingKeys(n.Pipe, vars)
		prePopulateMissingKeys(n.List, vars)
		prePopulateMissingKeys(n.ElseList, vars)
	case *parse.WithNode:
		prePopulateMissingKeys(n.Pipe, vars)
		prePopulateMissingKeys(n.List, vars)
		prePopulateMissingKeys(n.ElseList, vars)
	}
}

func RenderEmail(subject, html, text string, vars map[string]any) (Rendered, error) {
	var res Rendered
	var err error
	if subject != "" {
		res.Subject, err = Render(subject, vars)
		if err != nil {
			return Rendered{}, err
		}
	}
	if html != "" {
		res.HTML, err = Render(html, vars)
		if err != nil {
			return Rendered{}, err
		}
	}
	if text != "" {
		res.Text, err = Render(text, vars)
		if err != nil {
			return Rendered{}, err
		}
	}
	return res, nil
}

func isAbsoluteHTTP(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil {
		return false
	}
	return u.Host != "" && (strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https"))
}

func RewriteLinks(htmlStr, baseURL string, makeCode func(target string) (string, error)) (string, error) {
	z := xhtml.NewTokenizer(strings.NewReader(htmlStr))
	var out strings.Builder

	for {
		tt := z.Next()
		if tt == xhtml.ErrorToken {
			err := z.Err()
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("tokenize html: %w", err)
		}

		t := z.Token()
		if t.Type == xhtml.StartTagToken || t.Type == xhtml.SelfClosingTagToken {
			if t.Data == "a" {
				modified := false
				for i, attr := range t.Attr {
					if attr.Key == "href" {
						if isAbsoluteHTTP(attr.Val) {
							href := attr.Val
							code, err := makeCode(href)
							if err != nil {
								return "", fmt.Errorf("create tracking code for %q: %w", href, err)
							}
							base := strings.TrimSuffix(baseURL, "/")
							t.Attr[i].Val = base + "/t/" + code
							modified = true
						}
					}
				}
				if modified {
					out.WriteString(t.String())
					continue
				}
			}
		}

		// Use raw bytes to preserve original encoding/comments/scripts perfectly
		out.Write(z.Raw())
	}

	return out.String(), nil
}

func InjectPixel(html, baseURL, code string) string {
	urlStr := strings.TrimSuffix(baseURL, "/") + "/o/" + code + ".png"
	pixel := fmt.Sprintf(`<img src="%s" width="1" height="1" alt="" style="display:none" />`, urlStr)

	lower := strings.ToLower(html)
	idx := strings.Index(lower, "</body>")
	if idx != -1 {
		return html[:idx] + pixel + html[idx:]
	}
	return html + pixel
}

// InjectUnsubscribeFooter appends a compliance unsubscribe footer that links to
// {baseURL}/u/{code}. It is inserted before </body> when present, else
// appended. The code is a per-contact opt-out token. If code is empty the html
// is returned unchanged (no footer can be built).
func InjectUnsubscribeFooter(html, baseURL, code string) string {
	if code == "" {
		return html
	}
	urlStr := strings.TrimSuffix(baseURL, "/") + "/u/" + code
	footer := fmt.Sprintf(
		`<div style="margin-top:24px;padding-top:12px;border-top:1px solid #e5e5e5;`+
			`font-size:12px;color:#888;text-align:center">`+
			`<a href="%s" style="color:#888">Unsubscribe</a> from these emails.</div>`,
		urlStr)

	lower := strings.ToLower(html)
	idx := strings.Index(lower, "</body>")
	if idx != -1 {
		return html[:idx] + footer + html[idx:]
	}
	return html + footer
}
