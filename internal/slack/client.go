package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// Client represents a Slack API client
type Client struct {
	botToken  string
	channelID string
	client    *http.Client
}

// NewClient creates a new Slack client
func NewClient(botToken, channelID string) *Client {
	return &Client{
		botToken:  botToken,
		channelID: channelID,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SetStatus sets the user's Slack status
func (c *Client) SetStatus(statusText, statusEmoji string, expirationMinutes int) error {
	url := "https://slack.com/api/users.profile.set"

	expiration := time.Now().Add(time.Duration(expirationMinutes) * time.Minute).Unix()

	profile := map[string]interface{}{
		"status_text":       statusText,
		"status_emoji":      statusEmoji,
		"status_expiration": expiration,
	}

	payload := map[string]interface{}{
		"profile": profile,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal status payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create status request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.botToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to set status: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode status response: %w", err)
	}

	if ok, exists := result["ok"].(bool); !exists || !ok {
		errorMsg := "unknown error"
		if errStr, exists := result["error"].(string); exists {
			errorMsg = errStr
		}
		return fmt.Errorf("slack API error: %s", errorMsg)
	}

	log.Debug().
		Str("status_text", statusText).
		Str("status_emoji", statusEmoji).
		Int("expiration_minutes", expirationMinutes).
		Msg("Slack status updated")

	return nil
}

// PostMessage posts a message to the configured channel
func (c *Client) PostMessage(text string) error {
	url := "https://slack.com/api/chat.postMessage"

	payload := map[string]interface{}{
		"channel": c.channelID,
		"text":    text,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create message request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.botToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode message response: %w", err)
	}

	if ok, exists := result["ok"].(bool); !exists || !ok {
		errorMsg := "unknown error"
		if errStr, exists := result["error"].(string); exists {
			errorMsg = errStr
		}
		return fmt.Errorf("slack API error: %s", errorMsg)
	}

	log.Debug().
		Str("channel", c.channelID).
		Str("text", text).
		Msg("Message posted to Slack")

	return nil
}

// ClearStatus clears the user's Slack status
func (c *Client) ClearStatus() error {
	return c.SetStatus("", "", 0)
}
