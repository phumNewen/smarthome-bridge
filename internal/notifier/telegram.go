package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// TelegramClient sends messages via the Telegram Bot API.
type TelegramClient struct {
	baseURL       string
	botToken      string
	retryMax      int
	retryBackoff  []time.Duration
	httpClient    *http.Client
}

// TelegramConfig holds configuration for the Telegram client.
type TelegramConfig struct {
	APIBaseURL     string
	BotToken       string
	RetryMax       int
	RetryBackoffMs []int
}

// NewTelegramClient creates a new Telegram client.
func NewTelegramClient(cfg TelegramConfig) *TelegramClient {
	backoff := make([]time.Duration, len(cfg.RetryBackoffMs))
	for i, ms := range cfg.RetryBackoffMs {
		backoff[i] = time.Duration(ms) * time.Millisecond
	}

	return &TelegramClient{
		baseURL:      cfg.APIBaseURL,
		botToken:     cfg.BotToken,
		retryMax:     cfg.RetryMax,
		retryBackoff: backoff,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// telegramRequest is the JSON body sent to the sendMessage endpoint.
type telegramRequest struct {
	ChatID    int64  `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// telegramResponse is the JSON response from the Telegram API.
type telegramResponse struct {
	OK          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

// Send delivers a notification to all specified chat IDs.
// Returns the last error encountered (if any), but continues sending to remaining chats.
func (t *TelegramClient) Send(ctx context.Context, n Notification) error {
	var lastErr error
	for _, chatID := range n.ChatIDs {
		if err := t.sendToOne(ctx, chatID, n.Text, n.ParseMode); err != nil {
			slog.Error("telegram send failed",
				"chat_id", chatID,
				"error", err,
			)
			lastErr = err
			// Continue to next chat_id even if one fails.
		}
	}
	return lastErr
}

// sendToOne sends a message to a single chat with retry logic.
func (t *TelegramClient) sendToOne(ctx context.Context, chatID int64, text, parseMode string) error {
	reqBody := telegramRequest{
		ChatID:    chatID,
		Text:      text,
		ParseMode: parseMode,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", t.baseURL, t.botToken)

	var lastErr error
	maxAttempts := t.retryMax + 1 // retryMax retries + 1 initial attempt.

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Apply backoff before retry.
			backoffIdx := attempt - 1
			if backoffIdx < len(t.retryBackoff) {
				delay := t.retryBackoff[backoffIdx]
				slog.Debug("telegram retry",
					"chat_id", chatID,
					"attempt", attempt,
					"delay_ms", delay.Milliseconds(),
				)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := t.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)
			continue
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()

		var tgResp telegramResponse
		if err := json.Unmarshal(body, &tgResp); err != nil {
			lastErr = fmt.Errorf("unmarshal response (status %d): %w", resp.StatusCode, err)
			continue
		}

		if tgResp.OK {
			return nil
		}

		lastErr = fmt.Errorf("telegram error %d: %s", tgResp.ErrorCode, tgResp.Description)

		// Don't retry on client errors (4xx except 429).
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			return lastErr
		}
	}

	return fmt.Errorf("telegram send failed after %d attempts: %w", maxAttempts, lastErr)
}

// SendMessage is a convenience method that sends formatted HTML text to multiple chats.
func (t *TelegramClient) SendMessage(ctx context.Context, chatIDs []int64, text string) error {
	return t.Send(ctx, Notification{
		ChatIDs:   chatIDs,
		Text:      text,
		ParseMode: "HTML",
	})
}
