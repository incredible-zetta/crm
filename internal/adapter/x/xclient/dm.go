package xclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const dmNewURL = "https://x.com/i/api/1.1/dm/new2.json"

// DMOptions configures a direct message send.
type DMOptions struct {
	RecipientID string
	Text        string
	MediaID     string
}

// DMResult holds the outcome of a successful DM send.
type DMResult struct {
	MessageID      string
	ConversationID string
}

type dmNewResp struct {
	Entries []struct {
		Message struct {
			MessageData struct {
				ID   string `json:"id"`
				Text string `json:"text"`
			} `json:"message_data"`
		} `json:"message"`
	} `json:"entries"`
}

// MeUserID returns the authenticated account's numeric user id from the twid cookie.
func (c *Client) MeUserID() (string, error) {
	twid, err := c.jar.MustGet("twid")
	if err != nil {
		return "", err
	}
	decoded, err := url.QueryUnescape(twid)
	if err != nil {
		return "", fmt.Errorf("decode twid: %w", err)
	}
	id := strings.TrimPrefix(decoded, "u=")
	if id == "" || id == decoded {
		return "", fmt.Errorf("unexpected twid cookie format")
	}
	return id, nil
}

// ConversationID builds the 1:1 DM conversation id for a recipient.
func (c *Client) ConversationID(recipientID string) (string, error) {
	me, err := c.MeUserID()
	if err != nil {
		return "", err
	}
	return recipientID + "-" + me, nil
}

// SendDM sends a direct message via POST /i/api/1.1/dm/new2.json.
func (c *Client) SendDM(opts DMOptions) (*DMResult, error) {
	if opts.RecipientID == "" {
		return nil, fmt.Errorf("recipient id is required")
	}
	if opts.Text == "" && opts.MediaID == "" {
		return nil, fmt.Errorf("text or media is required")
	}

	convID, err := c.ConversationID(opts.RecipientID)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"cards_platform":      "Web-12",
		"conversation_id":     convID,
		"dm_users":            false,
		"include_cards":       1,
		"include_quote_count": true,
		"recipient_ids":       false,
		"text":                opts.Text,
	}
	if opts.MediaID != "" {
		payload["media_id"] = opts.MediaID
	}

	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, dmNewURL, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req)
	req.Header.Set("referer", "https://x.com/messages")
	if err := c.ensureTxn(); err != nil {
		return nil, fmt.Errorf("transaction id: %w", err)
	}
	c.setTransactionID(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	var out dmNewResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(out.Entries) == 0 || out.Entries[0].Message.MessageData.ID == "" {
		return nil, fmt.Errorf("no message id in response: %s", truncate(string(body), 300))
	}
	return &DMResult{
		MessageID:      out.Entries[0].Message.MessageData.ID,
		ConversationID: convID,
	}, nil
}

// SendDMToHandle resolves a @handle and sends a DM.
func (c *Client) SendDMToHandle(handle string, text, mediaPath string) (*DMResult, error) {
	u, err := c.UserByScreenName(strings.TrimPrefix(handle, "@"))
	if err != nil {
		return nil, err
	}
	opts := DMOptions{RecipientID: u.RestID, Text: text}
	if mediaPath != "" {
		id, err := c.UploadDMMedia(mediaPath)
		if err != nil {
			return nil, fmt.Errorf("upload dm media: %w", err)
		}
		opts.MediaID = id
	}
	return c.SendDM(opts)
}
