package xclient

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

const uploadBase = "https://upload.x.com/i/media/upload.json"

// mediaInitResp is the INIT command response.
type mediaInitResp struct {
	MediaID       int64  `json:"media_id"`
	MediaIDString string `json:"media_id_string"`
	MediaKey      string `json:"media_key"`
	ExpiresAfter  int    `json:"expires_after_secs"`
}

// setUploadHeaders applies auth headers for upload.x.com (no content-type; set
// per-request for multipart).
func (c *Client) setUploadHeaders(req *http.Request) {
	req.Header.Set("authorization", "Bearer "+decodeBearer(BearerToken))
	req.Header.Set("x-csrf-token", c.csrfToken)
	req.Header.Set("x-twitter-auth-type", "OAuth2Session")
	req.Header.Set("x-twitter-active-user", "yes")
	req.Header.Set("user-agent", userAgent)
	req.Header.Set("origin", "https://x.com")
	req.Header.Set("referer", "https://x.com/")
	req.Header.Set("cookie", c.jar.Header())
	_ = c.ensureTxn() // best-effort
	c.setTransactionID(req)
}

// UploadMedia performs the chunked INIT/APPEND/FINALIZE flow observed from the
// web client and returns the media_id string for use in CreateTweet.
func (c *Client) UploadMedia(path string) (string, error) {
	return c.uploadMedia(path, categoryFor)
}

// UploadDMMedia uploads an image for use in a direct message (dm_image category).
func (c *Client) UploadDMMedia(path string) (string, error) {
	return c.uploadMedia(path, dmCategoryFor)
}

func (c *Client) uploadMedia(path string, categoryFn func(string) string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read media: %w", err)
	}
	mediaType := mimeByExt(path)
	category := categoryFn(mediaType)

	mediaID, err := c.mediaInit(len(data), mediaType, category)
	if err != nil {
		return "", fmt.Errorf("INIT: %w", err)
	}
	if err := c.mediaAppend(mediaID, 0, data); err != nil {
		return "", fmt.Errorf("APPEND: %w", err)
	}
	if err := c.mediaFinalize(mediaID, data); err != nil {
		return "", fmt.Errorf("FINALIZE: %w", err)
	}
	return mediaID, nil
}

func (c *Client) mediaInit(totalBytes int, mediaType, category string) (string, error) {
	u := fmt.Sprintf("%s?command=INIT&total_bytes=%d&media_type=%s&media_category=%s",
		uploadBase, totalBytes, urlEscape(mediaType), category)
	req, err := http.NewRequest(http.MethodPost, u, nil)
	if err != nil {
		return "", err
	}
	c.setUploadHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(body), 300))
	}
	var out mediaInitResp
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("decode init: %w", err)
	}
	return out.MediaIDString, nil
}

func (c *Client) mediaAppend(mediaID string, segIndex int, chunk []byte) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("media", "blob")
	if err != nil {
		return err
	}
	if _, err := part.Write(chunk); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	u := fmt.Sprintf("%s?command=APPEND&media_id=%s&segment_index=%d",
		uploadBase, mediaID, segIndex)
	req, err := http.NewRequest(http.MethodPost, u, &buf)
	if err != nil {
		return err
	}
	c.setUploadHeaders(req)
	req.Header.Set("content-type", w.FormDataContentType())
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(b), 300))
	}
	return nil
}

func (c *Client) mediaFinalize(mediaID string, data []byte) error {
	sum := md5.Sum(data)
	u := fmt.Sprintf("%s?command=FINALIZE&media_id=%s&original_md5=%x",
		uploadBase, mediaID, sum)
	req, err := http.NewRequest(http.MethodPost, u, nil)
	if err != nil {
		return err
	}
	c.setUploadHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(b), 300))
	}
	return nil
}

func mimeByExt(path string) string {
	switch filepath.Ext(path) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}

func categoryFor(mediaType string) string {
	switch mediaType {
	case "video/mp4":
		return "tweet_video"
	case "image/gif":
		return "tweet_gif"
	default:
		return "tweet_image"
	}
}

func dmCategoryFor(mediaType string) string {
	switch mediaType {
	case "video/mp4":
		return "dm_video"
	case "image/gif":
		return "dm_gif"
	default:
		return "dm_image"
	}
}

// urlEscape escapes a value for use in a query string (e.g. "image/png").
func urlEscape(s string) string {
	return url.QueryEscape(s)
}
