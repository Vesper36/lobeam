package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Service handles external platform integrations via webhooks
type Service struct {
	slackWebhook  string
	zoomWebhook   string
	googleWebhook string
	publicURL     string
	httpClient    *http.Client
}

func NewService(slackWebhook, zoomWebhook, googleWebhook, publicURL string) *Service {
	return &Service{
		slackWebhook:  slackWebhook,
		zoomWebhook:   zoomWebhook,
		googleWebhook: googleWebhook,
		publicURL:     publicURL,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
	}
}

// SharePayload contains the data for a file share notification
type SharePayload struct {
	TransferID  string
	FileName    string
	FileSize    int64
	SenderName  string
	DownloadURL string
	Message     string
}

// SendToSlack posts a file share notification to a Slack incoming webhook
func (s *Service) SendToSlack(payload SharePayload) error {
	if s.slackWebhook == "" {
		return fmt.Errorf("slack webhook not configured")
	}
	if payload.DownloadURL == "" {
		payload.DownloadURL = fmt.Sprintf("%s/d/%s", s.publicURL, payload.TransferID)
	}

	body := map[string]interface{}{
		"blocks": []map[string]interface{}{
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*%s* shared a file", payload.SenderName),
				},
			},
			{
				"type": "section",
				"fields": []map[string]string{
					{"type": "mrkdwn", "text": fmt.Sprintf("*File:* %s", payload.FileName)},
					{"type": "mrkdwn", "text": fmt.Sprintf("*Size:* %s", formatBytes(payload.FileSize))},
				},
			},
			{
				"type": "actions",
				"elements": []map[string]interface{}{
					{
						"type":      "button",
						"text":      map[string]string{"type": "plain_text", "text": "Download"},
						"url":       payload.DownloadURL,
						"action_id": "download_file",
					},
				},
			},
		},
	}
	if payload.Message != "" {
		body["blocks"] = append(body["blocks"].([]map[string]interface{}), map[string]interface{}{
			"type": "context",
			"elements": []map[string]string{
				{"type": "mrkdwn", "text": payload.Message},
			},
		})
	}

	return s.postJSON(s.slackWebhook, body)
}

// SendToZoom posts a file share notification to a Zoom incoming webhook
func (s *Service) SendToZoom(payload SharePayload) error {
	if s.zoomWebhook == "" {
		return fmt.Errorf("zoom webhook not configured")
	}
	if payload.DownloadURL == "" {
		payload.DownloadURL = fmt.Sprintf("%s/d/%s", s.publicURL, payload.TransferID)
	}

	body := map[string]interface{}{
		"head": map[string]string{
			"text": fmt.Sprintf("File Shared: %s", payload.FileName),
		},
		"body": []map[string]interface{}{
			{
				"type": "message",
				"text": fmt.Sprintf("**%s** shared a file with you.\n\n**File:** %s\n**Size:** %s\n\n[Download File](%s)",
					payload.SenderName, payload.FileName, formatBytes(payload.FileSize), payload.DownloadURL),
			},
		},
	}
	return s.postJSON(s.zoomWebhook, body)
}

// SendToGoogleWorkspace posts a file share notification to a Google Chat webhook
func (s *Service) SendToGoogleWorkspace(payload SharePayload) error {
	if s.googleWebhook == "" {
		return fmt.Errorf("google webhook not configured")
	}
	if payload.DownloadURL == "" {
		payload.DownloadURL = fmt.Sprintf("%s/d/%s", s.publicURL, payload.TransferID)
	}

	body := map[string]interface{}{
		"cards": []map[string]interface{}{
			{
				"header": map[string]interface{}{
					"title":    fmt.Sprintf("File Shared: %s", payload.FileName),
					"subtitle": fmt.Sprintf("From %s", payload.SenderName),
				},
				"sections": []map[string]interface{}{
					{
						"widgets": []map[string]interface{}{
							{
								"textParagraph": map[string]string{
									"text": fmt.Sprintf("Size: %s", formatBytes(payload.FileSize)),
								},
							},
							{
								"buttons": []map[string]interface{}{
									{
										"textButton": map[string]interface{}{
											"text": "DOWNLOAD",
											"onClick": map[string]interface{}{
												"openLink": map[string]string{
													"url": payload.DownloadURL,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	if payload.Message != "" {
		sections := body["cards"].([]map[string]interface{})[0]["sections"].([]map[string]interface{})
		sections[0]["widgets"] = append(sections[0]["widgets"].([]map[string]interface{}), map[string]interface{}{
			"textParagraph": map[string]string{
				"text": fmt.Sprintf("<i>%s</i>", payload.Message),
			},
		})
	}
	return s.postJSON(s.googleWebhook, body)
}

func (s *Service) postJSON(url string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	resp, err := s.httpClient.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("post webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
