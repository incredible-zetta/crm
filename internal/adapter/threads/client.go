package threads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

const defaultBaseURL = "https://graph.threads.net"

type Config struct {
	AccessToken string
	AppSecret   string
	UserID      string
	APIVersion  string
	BaseURL     string
}

type Client struct {
	cfg    Config
	http   *http.Client
	base   string
	userID string
}

var _ port.ThreadsGateway = (*Client)(nil)

func New(cfg Config) (*Client, error) {
	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("threads access token required")
	}
	if cfg.UserID == "" {
		cfg.UserID = "me"
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = "v1.0"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	return &Client{cfg: cfg, http: http.DefaultClient, base: strings.TrimRight(cfg.BaseURL, "/"), userID: cfg.UserID}, nil
}

func (c *Client) Profile(ctx context.Context) (domain.ThreadsProfile, []byte, error) {
	raw, err := c.get(ctx, "/"+c.userID, url.Values{"fields": {"id,username,name,threads_profile_picture_url,threads_biography"}})
	if err != nil {
		return domain.ThreadsProfile{}, nil, err
	}
	var p domain.ThreadsProfile
	if err := json.Unmarshal(raw, &p); err != nil {
		return domain.ThreadsProfile{}, raw, err
	}
	return p, raw, nil
}

func (c *Client) List(ctx context.Context, limit int, cursor string) ([]domain.ThreadsPost, string, error) {
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{"fields": {"id,media_product_type,media_type,text,permalink,timestamp,username,topic_tag,is_quote_post"}, "limit": {fmt.Sprint(limit)}}
	if cursor != "" {
		q.Set("after", cursor)
	}
	raw, err := c.get(ctx, "/"+c.userID+"/threads", q)
	if err != nil {
		return nil, "", err
	}
	var page graphPage[postDTO]
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, "", err
	}
	posts := make([]domain.ThreadsPost, 0, len(page.Data))
	for _, item := range page.Data {
		posts = append(posts, item.domain(rawJSON(item)))
	}
	return posts, page.Paging.Cursors.After, nil
}

func (c *Client) Publish(ctx context.Context, in port.ThreadsPublishInput) (port.ThreadsPublishResult, error) {
	if in.Text == "" && in.ImageURL == "" && in.VideoURL == "" {
		return port.ThreadsPublishResult{}, fmt.Errorf("text, image_url, or video_url required")
	}
	mediaType := "TEXT"
	if in.VideoURL != "" {
		mediaType = "VIDEO"
	} else if in.ImageURL != "" {
		mediaType = "IMAGE"
	}
	body := url.Values{"media_type": {mediaType}}
	if in.Text != "" {
		body.Set("text", in.Text)
	}
	if in.ImageURL != "" {
		body.Set("image_url", in.ImageURL)
	}
	if in.VideoURL != "" {
		body.Set("video_url", in.VideoURL)
	}
	if in.TopicTag != "" {
		body.Set("topic_tag", in.TopicTag)
	}
	containerRaw, err := c.post(ctx, "/"+c.userID+"/threads", body)
	if err != nil {
		return port.ThreadsPublishResult{}, err
	}
	var container struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(containerRaw, &container); err != nil {
		return port.ThreadsPublishResult{}, err
	}
	if mediaType == "VIDEO" {
		time.Sleep(30 * time.Second)
	}
	publishedRaw, err := c.post(ctx, "/"+c.userID+"/threads_publish", url.Values{"creation_id": {container.ID}})
	if err != nil {
		return port.ThreadsPublishResult{}, err
	}
	var published struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(publishedRaw, &published); err != nil {
		return port.ThreadsPublishResult{}, err
	}
	post, raw, err := c.fetchPost(ctx, published.ID)
	if err != nil {
		post = domain.ThreadsPost{ThreadsID: published.ID, RawJSON: publishedRaw}
		raw = publishedRaw
	}
	return port.ThreadsPublishResult{ContainerID: container.ID, Post: post, RawJSON: raw}, nil
}

func (c *Client) Delete(ctx context.Context, mediaID string) error {
	_, err := c.delete(ctx, "/"+mediaID, nil)
	return err
}

func (c *Client) Insights(ctx context.Context, mediaID string) ([]domain.ThreadsInsight, []byte, error) {
	path := "/" + c.userID + "/threads_insights"
	metrics := "views,likes,replies,reposts,quotes,followers_count"
	if mediaID != "" {
		path = "/" + mediaID + "/insights"
		metrics = "views,likes,replies,reposts,quotes,shares"
	}
	raw, err := c.get(ctx, path, url.Values{"metric": {metrics}})
	if err != nil {
		return nil, nil, err
	}
	var page struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, raw, err
	}
	items := make([]domain.ThreadsInsight, 0, len(page.Data))
	for _, m := range page.Data {
		items = append(items, domain.ThreadsInsight{Name: fmt.Sprint(m["name"]), RawValue: m})
	}
	return items, raw, nil
}

func (c *Client) Replies(ctx context.Context, mediaID string, limit int, cursor string) ([]domain.ThreadsReply, string, error) {
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{"fields": {"id,text,username,timestamp,hide_status"}, "limit": {fmt.Sprint(limit)}}
	if cursor != "" {
		q.Set("after", cursor)
	}
	raw, err := c.get(ctx, "/"+mediaID+"/replies", q)
	if err != nil {
		return nil, "", err
	}
	var page graphPage[replyDTO]
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, "", err
	}
	items := make([]domain.ThreadsReply, 0, len(page.Data))
	for _, r := range page.Data {
		items = append(items, r.domain(mediaID, rawJSON(r)))
	}
	return items, page.Paging.Cursors.After, nil
}

func (c *Client) Conversation(ctx context.Context, mediaID string, limit int, cursor string) ([]domain.ThreadsReply, string, error) {
	if limit <= 0 {
		limit = 50
	}
	q := url.Values{"fields": {"id,text,username,timestamp,hide_status,replied_to,root_post,is_reply,has_replies"}, "limit": {fmt.Sprint(limit)}}
	if cursor != "" {
		q.Set("after", cursor)
	}
	raw, err := c.get(ctx, "/"+mediaID+"/conversation", q)
	if err != nil {
		return nil, "", err
	}
	var page graphPage[replyDTO]
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, "", err
	}
	items := make([]domain.ThreadsReply, 0, len(page.Data))
	for _, r := range page.Data {
		items = append(items, r.domain(mediaID, rawJSON(r)))
	}
	return items, page.Paging.Cursors.After, nil
}

func (c *Client) Reply(ctx context.Context, mediaID, text string) (string, []byte, error) {
	containerRaw, err := c.post(ctx, "/"+c.userID+"/threads", url.Values{"media_type": {"TEXT"}, "text": {text}, "reply_to_id": {mediaID}})
	if err != nil {
		return "", nil, err
	}
	var container struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(containerRaw, &container); err != nil {
		return "", containerRaw, err
	}
	time.Sleep(5 * time.Second)
	publishedRaw, err := c.post(ctx, "/"+c.userID+"/threads_publish", url.Values{"creation_id": {container.ID}})
	if err != nil {
		return "", containerRaw, err
	}
	var published struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(publishedRaw, &published); err != nil {
		return "", publishedRaw, err
	}
	raw, _ := json.Marshal(map[string]json.RawMessage{
		"container": json.RawMessage(containerRaw),
		"publish":   json.RawMessage(publishedRaw),
	})
	return published.ID, raw, nil
}

func (c *Client) ReplyQuota(ctx context.Context) (map[string]any, []byte, error) {
	raw, err := c.get(ctx, "/"+c.userID+"/threads_publishing_limit", url.Values{"fields": {"reply_quota_usage,reply_config"}})
	if err != nil {
		return nil, nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, raw, err
	}
	return out, raw, nil
}

func (c *Client) Mentions(ctx context.Context, limit int, cursor string) ([]domain.ThreadsMention, string, error) {
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{"fields": {"id,text,username,timestamp,permalink"}, "limit": {fmt.Sprint(limit)}}
	if cursor != "" {
		q.Set("after", cursor)
	}
	raw, err := c.get(ctx, "/"+c.userID+"/mentions", q)
	if err != nil {
		return nil, "", err
	}
	var page graphPage[mentionDTO]
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, "", err
	}
	items := make([]domain.ThreadsMention, 0, len(page.Data))
	for _, m := range page.Data {
		items = append(items, m.domain(rawJSON(m)))
	}
	return items, page.Paging.Cursors.After, nil
}

func (c *Client) Search(ctx context.Context, in port.ThreadsSearchInput) (map[string]any, []byte, error) {
	if in.Limit <= 0 {
		in.Limit = 10
	}
	fields := in.Fields
	if fields == "" {
		fields = "id,text,media_type,permalink,timestamp,username,has_replies,is_quote_post,is_reply,topic_tag"
	}
	q := url.Values{"q": {in.Query}, "limit": {fmt.Sprint(in.Limit)}, "fields": {fields}}
	if in.SearchType != "" {
		q.Set("search_type", strings.ToUpper(in.SearchType))
	}
	if in.SearchMode != "" {
		q.Set("search_mode", strings.ToUpper(in.SearchMode))
	}
	if in.MediaType != "" {
		q.Set("media_type", strings.ToUpper(in.MediaType))
	}
	if in.AuthorUsername != "" {
		q.Set("author_username", strings.TrimPrefix(in.AuthorUsername, "@"))
	}
	if in.Since != "" {
		q.Set("since", in.Since)
	}
	if in.Until != "" {
		q.Set("until", in.Until)
	}
	if in.Cursor != "" {
		q.Set("after", in.Cursor)
	}
	raw, err := c.get(ctx, "/keyword_search", q)
	if err != nil {
		return nil, nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, raw, err
	}
	return out, raw, nil
}

func (c *Client) ExchangeToken(ctx context.Context, accessToken string) (port.ThreadsTokenResult, error) {
	if accessToken == "" {
		accessToken = c.cfg.AccessToken
	}
	if c.cfg.AppSecret == "" {
		return port.ThreadsTokenResult{}, fmt.Errorf("threads app secret required")
	}
	return c.tokenRequest(ctx, "/access_token", url.Values{"grant_type": {"th_exchange_token"}, "client_secret": {c.cfg.AppSecret}, "access_token": {accessToken}})
}

func (c *Client) RefreshToken(ctx context.Context, accessToken string) (port.ThreadsTokenResult, error) {
	if accessToken == "" {
		accessToken = c.cfg.AccessToken
	}
	return c.tokenRequest(ctx, "/refresh_access_token", url.Values{"grant_type": {"th_refresh_token"}, "access_token": {accessToken}})
}

func (c *Client) tokenRequest(ctx context.Context, path string, q url.Values) (port.ThreadsTokenResult, error) {
	u, err := url.Parse(c.base + path)
	if err != nil {
		return port.ThreadsTokenResult{}, err
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return port.ThreadsTokenResult{}, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return port.ThreadsTokenResult{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return port.ThreadsTokenResult{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return port.ThreadsTokenResult{}, fmt.Errorf("threads API %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out port.ThreadsTokenResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return port.ThreadsTokenResult{}, err
	}
	out.RawJSON = raw
	return out, nil
}

func (c *Client) fetchPost(ctx context.Context, id string) (domain.ThreadsPost, []byte, error) {
	raw, err := c.get(ctx, "/"+id, url.Values{"fields": {"id,media_product_type,media_type,text,permalink,timestamp,username,topic_tag,is_quote_post"}})
	if err != nil {
		return domain.ThreadsPost{}, nil, err
	}
	var dto postDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		return domain.ThreadsPost{}, raw, err
	}
	return dto.domain(raw), raw, nil
}

func (c *Client) get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, q, nil)
}

func (c *Client) post(ctx context.Context, path string, form url.Values) ([]byte, error) {
	return c.do(ctx, http.MethodPost, path, nil, form)
}

func (c *Client) delete(ctx context.Context, path string, q url.Values) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, path, q, nil)
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, form url.Values) ([]byte, error) {
	if q == nil {
		q = url.Values{}
	}
	q.Set("access_token", c.cfg.AccessToken)
	u, err := url.Parse(c.base + "/" + strings.Trim(c.cfg.APIVersion, "/") + path)
	if err != nil {
		return nil, err
	}
	u.RawQuery = q.Encode()
	var body io.Reader
	headers := http.Header{}
	if form != nil {
		form.Set("access_token", c.cfg.AccessToken)
		body = bytes.NewBufferString(form.Encode())
		headers.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header = headers
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		msg := res.Status
		if e, ok := body["error"].(map[string]any); ok && e["message"] != nil {
			msg = fmt.Sprint(e["message"])
		}
		return nil, fmt.Errorf("threads API %d: %s", res.StatusCode, msg)
	}
	return raw, nil
}

type graphPage[T any] struct {
	Data   []T `json:"data"`
	Paging struct {
		Cursors struct {
			After string `json:"after"`
		} `json:"cursors"`
	} `json:"paging"`
}

type postDTO struct {
	ID               string `json:"id"`
	MediaProductType string `json:"media_product_type"`
	MediaType        string `json:"media_type"`
	Text             string `json:"text"`
	Permalink        string `json:"permalink"`
	Timestamp        string `json:"timestamp"`
	Username         string `json:"username"`
	TopicTag         string `json:"topic_tag"`
	IsQuotePost      bool   `json:"is_quote_post"`
}

func (p postDTO) domain(raw []byte) domain.ThreadsPost {
	return domain.ThreadsPost{ThreadsID: p.ID, MediaProductType: p.MediaProductType, MediaType: p.MediaType, Text: p.Text, Permalink: p.Permalink, Timestamp: parseTimePtr(p.Timestamp), Username: p.Username, TopicTag: p.TopicTag, IsQuotePost: p.IsQuotePost, RawJSON: raw}
}

type replyDTO struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Username   string `json:"username"`
	Timestamp  string `json:"timestamp"`
	HideStatus string `json:"hide_status"`
	HasReplies bool   `json:"has_replies"`
	RepliedTo  *struct {
		ID string `json:"id"`
	} `json:"replied_to"`
}

func (r replyDTO) domain(postID string, raw []byte) domain.ThreadsReply {
	parentID := ""
	if r.RepliedTo != nil {
		parentID = r.RepliedTo.ID
	}
	return domain.ThreadsReply{ReplyID: r.ID, PostID: postID, ParentID: parentID, Text: r.Text, Username: r.Username, Timestamp: parseTimePtr(r.Timestamp), HideStatus: r.HideStatus, HasReplies: r.HasReplies, RawJSON: raw}
}

type mentionDTO struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Username  string `json:"username"`
	Permalink string `json:"permalink"`
	Timestamp string `json:"timestamp"`
}

func (m mentionDTO) domain(raw []byte) domain.ThreadsMention {
	return domain.ThreadsMention{MentionID: m.ID, Text: m.Text, Username: m.Username, Permalink: m.Permalink, Timestamp: parseTimePtr(m.Timestamp), RawJSON: raw}
}

func parseTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05-0700"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func rawJSON(v any) []byte {
	raw, _ := json.Marshal(v)
	return raw
}
