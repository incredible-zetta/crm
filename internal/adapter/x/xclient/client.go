package xclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/adapter/x/xtransaction"
)

// BearerToken is the public web-app bearer token used by x.com's GraphQL API.
// It is not a secret; it ships in the site's JS bundle and is identical for all
// unauthenticated and authenticated web sessions.
const BearerToken = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"

const (
	apiBase   = "https://x.com/i/api/graphql"
	userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"
)

// Client is an authenticated x.com GraphQL client backed by browser cookies.
type Client struct {
	http      *http.Client
	jar       *CookieJar
	authToken string
	csrfToken string // == ct0 cookie
	lang      string
	txn       *xtransaction.Client // lazily built; nil until first ensureTxn
}

// NewClientFromCookies builds a Client from an in-memory Netscape cookie blob.
// Use this for per-account auth passed at call time (multi-account) instead of
// a fixed file on disk. Requires auth_token and ct0 cookies.
func NewClientFromCookies(cookies string) (*Client, error) {
	jar, err := ParseNetscapeCookies(strings.NewReader(cookies))
	if err != nil {
		return nil, err
	}
	return newClientFromJar(jar)
}

// NewClientFromCookieFile builds a Client from a Netscape cookie file. It
// requires auth_token and ct0 cookies to be present.
func NewClientFromCookieFile(path string) (*Client, error) {
	jar, err := ParseNetscapeCookieFile(path)
	if err != nil {
		return nil, err
	}
	return newClientFromJar(jar)
}

func newClientFromJar(jar *CookieJar) (*Client, error) {
	authToken, err := jar.MustGet("auth_token")
	if err != nil {
		return nil, err
	}
	ct0, err := jar.MustGet("ct0")
	if err != nil {
		return nil, err
	}
	lang, _ := jar.Get("lang")
	if lang == "" {
		lang = "en"
	}
	return &Client{
		http:      &http.Client{Timeout: 30 * time.Second},
		jar:       jar,
		authToken: authToken,
		csrfToken: ct0,
		lang:      lang,
	}, nil
}

// setAuthHeaders applies the auth scheme observed from the web client:
//   - Authorization: Bearer <public token>
//   - x-csrf-token: <ct0>
//   - x-twitter-auth-type: OAuth2Session
//   - Cookie: full jar (auth_token + ct0 + ...)
func (c *Client) setAuthHeaders(req *http.Request) {
	req.Header.Set("authorization", "Bearer "+decodeBearer(BearerToken))
	req.Header.Set("x-csrf-token", c.csrfToken)
	req.Header.Set("x-twitter-auth-type", "OAuth2Session")
	req.Header.Set("x-twitter-active-user", "yes")
	req.Header.Set("x-twitter-client-language", c.lang)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "*/*")
	req.Header.Set("user-agent", userAgent)
	req.Header.Set("cookie", c.jar.Header())
}

// decodeBearer un-escapes the %3D that appears in the captured token so the
// wire value matches what the browser sends.
func decodeBearer(t string) string {
	return strings.ReplaceAll(t, "%3D", "=")
}

// ensureTxn lazily builds the x-client-transaction-id generator by fetching the
// authenticated home page and its ondemand.s JS file. Safe to call repeatedly;
// builds once. A failure is non-fatal to callers: they may proceed without the
// header (x.com currently accepts requests lacking it, but sending it matches
// the browser and hardens against future enforcement).
func (c *Client) ensureTxn() error {
	if c.txn != nil {
		return nil
	}
	homeHTML, err := c.fetchText(http.MethodGet, "https://x.com/home", "https://x.com/")
	if err != nil {
		return fmt.Errorf("fetch home page: %w", err)
	}
	jsURL, err := xtransaction.OnDemandFileURL(homeHTML)
	if err != nil {
		return err
	}
	js, err := c.fetchText(http.MethodGet, jsURL, "https://x.com/")
	if err != nil {
		return fmt.Errorf("fetch ondemand js: %w", err)
	}
	txn, err := xtransaction.New(homeHTML, js)
	if err != nil {
		return err
	}
	c.txn = txn
	return nil
}

// fetchText performs a GET and returns the response body as a string with auth
// headers applied (needed so the home page includes the verification key).
func (c *Client) fetchText(method, rawURL, referer string) (string, error) {
	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		return "", err
	}
	// HTML page fetches must not include the API bearer token; x.com returns
	// 401 when both cookies and bearer are sent on document requests.
	req.Header.Set("user-agent", userAgent)
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("cookie", c.jar.Header())
	req.Header.Set("referer", referer)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http %d", resp.StatusCode)
	}
	return string(body), nil
}

// setTransactionID computes and sets the x-client-transaction-id header for the
// given request. No-op if the generator is unavailable.
func (c *Client) setTransactionID(req *http.Request) {
	if c.txn == nil {
		return
	}
	tid := c.txn.GenerateTransactionID(req.Method, req.URL.Path)
	req.Header.Set("x-client-transaction-id", tid)
}

// GraphQLError represents an error entry in a GraphQL response.
type GraphQLError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func (e GraphQLError) Error() string {
	return fmt.Sprintf("graphql error (code %d): %s", e.Code, e.Message)
}

// doGraphQL issues a GET GraphQL query with variables + features maps and
// decodes the JSON response into out.
func (c *Client) doGraphQL(queryID, opName string, variables, features, fieldToggles map[string]any, out any) error {
	u, err := url.Parse(fmt.Sprintf("%s/%s/%s", apiBase, queryID, opName))
	if err != nil {
		return err
	}
	q := u.Query()
	if variables != nil {
		b, _ := json.Marshal(variables)
		q.Set("variables", string(b))
	}
	if features != nil {
		b, _ := json.Marshal(features)
		q.Set("features", string(b))
	}
	if fieldToggles != nil {
		b, _ := json.Marshal(fieldToggles)
		q.Set("fieldToggles", string(b))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	c.setAuthHeaders(req)
	req.Header.Set("referer", "https://x.com/")
	_ = c.ensureTxn() // best-effort; header omitted on failure
	c.setTransactionID(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	// Check for GraphQL-level errors before decoding into out.
	var errEnvelope struct {
		Errors []GraphQLError `json:"errors"`
	}
	if err := json.Unmarshal(body, &errEnvelope); err == nil && len(errEnvelope.Errors) > 0 {
		return errEnvelope.Errors[0]
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
