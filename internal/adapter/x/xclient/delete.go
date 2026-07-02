package xclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const deleteTweetQueryID = "nxpZCY2K-I6QoFHAHeojFQ"

type deleteTweetResp struct {
	Data struct {
		DeleteTweet struct {
			TweetResults struct {
				Result struct {
					RestID string `json:"rest_id"`
				} `json:"result"`
			} `json:"tweet_results"`
		} `json:"delete_tweet"`
	} `json:"data"`
	Errors []GraphQLError `json:"errors"`
}

// DeleteTweet removes a tweet owned by the authenticated account.
func (c *Client) DeleteTweet(tweetID string) error {
	variables := map[string]any{
		"tweet_id":     tweetID,
		"dark_request": false,
	}
	payload := map[string]any{
		"variables": variables,
		"queryId":   deleteTweetQueryID,
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	u := fmt.Sprintf("%s/%s/DeleteTweet", apiBase, deleteTweetQueryID)
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	c.setAuthHeaders(req)
	req.Header.Set("referer", "https://x.com/home")
	if err := c.ensureTxn(); err != nil {
		return fmt.Errorf("transaction id: %w", err)
	}
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

	var out deleteTweetResp
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if len(out.Errors) > 0 {
		return out.Errors[0]
	}
	return nil
}
