package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func SendSlackMessage(hackatimeMessageQueueAPIKey string, userID string, message string, blocksJSON string) error {
	payload := map[string]any{
		"channel": userID,
		"text":    message,
		"blocks":  blocksJSON,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling payload: %v", err)
	}

	req, err := http.NewRequest("POST", "https://hackatime-bot.kierank.hackclub.app/slack/message", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+hackatimeMessageQueueAPIKey)
	req.Header.Set("User-Agent", "waka.hackclub.com (reset password)")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	var result struct {
		Records []struct{} `json:"records"`
		Error   struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("error parsing response: %v", err)
	}

	if result.Error.Type != "" {
		return fmt.Errorf("Hackatime message queue error: %s - %s", result.Error.Type, result.Error.Message)
	}

	return nil
}
