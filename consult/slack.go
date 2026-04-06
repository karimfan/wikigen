package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type slackReply struct {
	UserID string
	Text   string
	TS     string
}

type consultConfig struct {
	UserMap        map[string]string `json:"user_map"`
	DefaultChannel string            `json:"default_channel"`
}

func loadConsultConfig(repoRoot string) *consultConfig {
	path := filepath.Join(repoRoot, ".consult.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg consultConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

func requireSlackToken() string {
	token := os.Getenv("SLACK_BOT_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, `error: SLACK_BOT_TOKEN environment variable is not set

Create a Slack app at https://api.slack.com/apps with these scopes:
  - chat:write      (send messages)
  - users:read.email (look up users by email)
  - im:write         (open DM channels)

Then export SLACK_BOT_TOKEN=xoxb-...`)
		os.Exit(1)
	}
	return token
}

func slackGet(token, apiURL string, params url.Values) ([]byte, error) {
	if params != nil {
		apiURL = apiURL + "?" + params.Encode()
	}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("slack API request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("slack API returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func slackPost(token, apiURL, body string) ([]byte, error) {
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("slack API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return respBody, fmt.Errorf("slack API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func lookupSlackUser(token, email string) (userID, displayName string, err error) {
	params := url.Values{"email": {email}}
	body, err := slackGet(token, "https://slack.com/api/users.lookupByEmail", params)
	if err != nil {
		return "", "", err
	}

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		User  struct {
			ID      string `json:"id"`
			Profile struct {
				DisplayName string `json:"display_name"`
				RealName    string `json:"real_name"`
			} `json:"profile"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("parsing response: %w", err)
	}
	if !result.OK {
		return "", "", fmt.Errorf("slack API error: %s", result.Error)
	}

	name := result.User.Profile.DisplayName
	if name == "" {
		name = result.User.Profile.RealName
	}
	return result.User.ID, name, nil
}

func openDM(token, userID string) (channelID string, err error) {
	payload := fmt.Sprintf(`{"users":"%s"}`, userID)
	body, err := slackPost(token, "https://slack.com/api/conversations.open", payload)
	if err != nil {
		return "", err
	}

	var result struct {
		OK      bool   `json:"ok"`
		Error   string `json:"error"`
		Channel struct {
			ID string `json:"id"`
		} `json:"channel"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}
	if !result.OK {
		return "", fmt.Errorf("slack API error: %s", result.Error)
	}
	return result.Channel.ID, nil
}

func postMessage(token, channel, text string) (threadTS string, err error) {
	payload, _ := json.Marshal(map[string]string{
		"channel": channel,
		"text":    text,
	})
	body, err := slackPost(token, "https://slack.com/api/chat.postMessage", string(payload))
	if err != nil {
		return "", err
	}

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		TS    string `json:"ts"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}
	if !result.OK {
		return "", fmt.Errorf("slack API error: %s", result.Error)
	}
	return result.TS, nil
}

func getThreadReplies(token, channel, threadTS string) ([]slackReply, error) {
	params := url.Values{
		"channel": {channel},
		"ts":      {threadTS},
	}
	body, err := slackGet(token, "https://slack.com/api/conversations.replies", params)
	if err != nil {
		return nil, err
	}

	var result struct {
		OK       bool   `json:"ok"`
		Error    string `json:"error"`
		Messages []struct {
			User string `json:"user"`
			Text string `json:"text"`
			TS   string `json:"ts"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("slack API error: %s", result.Error)
	}

	// Skip the first message (the original post)
	var replies []slackReply
	for i, msg := range result.Messages {
		if i == 0 {
			continue
		}
		replies = append(replies, slackReply{
			UserID: msg.User,
			Text:   msg.Text,
			TS:     msg.TS,
		})
	}
	return replies, nil
}

func resolveExpertSlackIDs(token, repoRoot string, experts []expert) []expert {
	cfg := loadConsultConfig(repoRoot)

	for i := range experts {
		email := experts[i].Email

		// Try config first
		if cfg != nil && cfg.UserMap != nil {
			if slackID, ok := cfg.UserMap[email]; ok {
				experts[i].SlackID = slackID
				continue
			}
		}

		// Fall back to Slack API lookup
		if token != "" {
			userID, _, err := lookupSlackUser(token, email)
			if err == nil && userID != "" {
				experts[i].SlackID = userID
			}
		}
	}
	return experts
}
