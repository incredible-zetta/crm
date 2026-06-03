package template

import (
	"fmt"
	"io"
	"net/url"
	"strings"
	texttemplate "text/template"
	"text/template/parse"

	xhtml "golang.org/x/net/html"
)

type Rendered struct {
	Subject, HTML, Text string
}

func Render(tmpl string, vars map[string]any) (string, error) {
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

	res := buf.String()
	res = strings.ReplaceAll(res, "<no value>", "")
	return res, nil
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
		if len(n.Ident) > 0 {
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
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Host != ""
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
			return "", err
		}

		t := z.Token()
		if t.Type == xhtml.StartTagToken || t.Type == xhtml.SelfClosingTagToken {
			if t.Data == "a" {
				modified := false
				for i, attr := range t.Attr {
					if attr.Key == "href" {
						if isAbsoluteHTTP(attr.Val) {
							code, err := makeCode(attr.Val)
							if err != nil {
								return "", err
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
