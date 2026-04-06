package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type session struct {
	ID           string   `json:"id"`
	CreatedAt    string   `json:"created_at"`
	File         string   `json:"file"`
	Question     string   `json:"question"`
	Type         string   `json:"type"`
	Status       string   `json:"status"`
	Response     string   `json:"response"`
	Experts      []expert `json:"experts"`
	SlackChannel string   `json:"slack_channel"`
	SlackThread  string   `json:"slack_thread_ts"`
}

func sessionDir(repoRoot string) string {
	return filepath.Join(repoRoot, ".consult")
}

func generateSessionID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("%08x", time.Now().UnixNano()&0xFFFFFFFF)
	}
	return hex.EncodeToString(b)
}

func createSession(repoRoot, file, question, sessionType string, experts []expert, channel, threadTS string) (session, error) {
	dir := sessionDir(repoRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return session{}, fmt.Errorf("creating session dir: %w", err)
	}

	s := session{
		ID:           generateSessionID(),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		File:         file,
		Question:     question,
		Type:         sessionType,
		Status:       "pending",
		Experts:      experts,
		SlackChannel: channel,
		SlackThread:  threadTS,
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return session{}, fmt.Errorf("marshaling session: %w", err)
	}

	path := filepath.Join(dir, s.ID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return session{}, fmt.Errorf("writing session file: %w", err)
	}

	return s, nil
}

func loadSession(repoRoot, sessionID string) (session, error) {
	path := filepath.Join(sessionDir(repoRoot), sessionID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return session{}, fmt.Errorf("reading session %s: %w", sessionID, err)
	}

	var s session
	if err := json.Unmarshal(data, &s); err != nil {
		return session{}, fmt.Errorf("parsing session %s: %w", sessionID, err)
	}
	return s, nil
}

func updateSession(repoRoot string, s session) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}

	path := filepath.Join(sessionDir(repoRoot), s.ID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}
	return nil
}

func checkSessionResponse(token string, s session) (string, bool, error) {
	replies, err := getThreadReplies(token, s.SlackChannel, s.SlackThread)
	if err != nil {
		return "", false, err
	}
	if len(replies) > 0 {
		return replies[0].Text, true, nil
	}
	return "", false, nil
}

func listSessions(repoRoot string) ([]session, error) {
	pattern := filepath.Join(sessionDir(repoRoot), "*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}

	var sessions []session
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var s session
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		sessions = append(sessions, s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt > sessions[j].CreatedAt
	})

	return sessions, nil
}

func printSessions(sessions []session) {
	fmt.Printf("\n%-10s %-30s %-10s %-10s %s\n",
		"ID", "File", "Type", "Status", "Created")
	fmt.Println(strings.Repeat("-", 80))
	for _, s := range sessions {
		fmt.Printf("%-10s %-30s %-10s %-10s %s\n",
			s.ID,
			truncate(s.File, 30),
			s.Type,
			s.Status,
			s.CreatedAt,
		)
	}
	fmt.Println()
}
