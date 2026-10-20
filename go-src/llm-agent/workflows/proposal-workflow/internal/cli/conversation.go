package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type conversationMessage struct{ Timestamp, Speaker, Text string }
type transcriptPart struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Name      string `json:"name"`
	ID        string `json:"id"`
	ToolUseID string `json:"tool_use_id"`
	IsError   bool   `json:"is_error"`
	Input     struct {
		Text *string `json:"text"`
	} `json:"input"`
}
type transcriptRecord struct {
	Type      string `json:"type"`
	Operation string `json:"operation"`
	Timestamp string `json:"timestamp"`
	Content   string `json:"content"`
	Message   *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	PromptSource string `json:"promptSource"`
	Origin       *struct {
		Kind string `json:"kind"`
	} `json:"origin"`
}

func exportConversation(args []string) error {
	if len(args) < 4 || args[0] != "export" {
		return fmt.Errorf("conversation export <session-transcript> <repository> <project-key> [YYYY-MM-DD ...]")
	}
	key := strings.Trim(args[3], "/")
	if !filepath.IsLocal(key) {
		return fmt.Errorf("invalid project key")
	}
	directory := filepath.Join(args[2], ".local/conversation", key)
	dates := args[4:]
	if len(dates) == 0 {
		now := time.Now()
		dates = []string{now.AddDate(0, 0, -1).Format("2006-01-02"), now.Format("2006-01-02")}
	}
	selected := map[string][]string{}
	for _, day := range dates {
		if _, err := time.Parse("2006-01-02", day); err != nil {
			return err
		}
		selected[day] = nil
	}
	file, err := os.Open(args[1])
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)
	records := []transcriptRecord{}
	failed := map[string]bool{}
	line := 0
	for scanner.Scan() {
		line++
		raw := scanner.Bytes()
		if !utf8.Valid(raw) || len(strings.TrimSpace(string(raw))) == 0 || strings.TrimSpace(string(raw))[0] != '{' {
			return fmt.Errorf("%s:%d: expected UTF-8 record object", args[1], line)
		}
		var record transcriptRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return fmt.Errorf("%s:%d: %w", args[1], line, err)
		}
		if record.Message != nil && len(record.Message.Content) > 0 && record.Message.Content[0] == '[' {
			var parts []transcriptPart
			if err := json.Unmarshal(record.Message.Content, &parts); err != nil {
				return fmt.Errorf("%s:%d: %w", args[1], line, err)
			}
			for _, part := range parts {
				if part.Type == "tool_result" && part.IsError {
					failed[part.ToolUseID] = true
				}
			}
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	channel := regexp.MustCompile(`(?s)<channel\b([^>]*)>\n?(.*?)</channel>`)
	timestamp := regexp.MustCompile(`\bts="([^"]+)"`)
	messages := []conversationMessage{}
	for index, record := range records {
		texts := []string{}
		if record.Type == "queue-operation" && record.Operation == "enqueue" {
			texts = append(texts, record.Content)
		}
		if record.Message != nil && len(record.Message.Content) > 0 {
			content := record.Message.Content
			if record.Type == "user" && content[0] == '"' {
				var text string
				if err := json.Unmarshal(content, &text); err != nil {
					return err
				}
				if record.Origin != nil && record.Origin.Kind == "human" && record.PromptSource == "typed" {
					if record.Timestamp == "" {
						return fmt.Errorf("%s:%d: typed user message has no timestamp", args[1], index+1)
					}
					messages = append(messages, conversationMessage{record.Timestamp, "USER", strings.TrimSpace(text)})
				}
				texts = append(texts, text)
			} else if content[0] == '[' {
				var parts []transcriptPart
				if err := json.Unmarshal(content, &parts); err != nil {
					return err
				}
				for _, part := range parts {
					if record.Type == "user" && part.Type == "text" {
						texts = append(texts, part.Text)
					}
					if record.Type == "assistant" && part.Type == "tool_use" && strings.Contains(part.Name, "telegram__reply") && !failed[part.ID] {
						if record.Timestamp == "" || part.Input.Text == nil {
							return fmt.Errorf("%s:%d: reply lacks timestamp or text", args[1], index+1)
						}
						messages = append(messages, conversationMessage{record.Timestamp, "ORCHESTRATOR", strings.TrimSpace(*part.Input.Text)})
					}
				}
			} else if record.Type == "user" && string(content) != "null" {
				return fmt.Errorf("%s:%d: expected user text or array", args[1], index+1)
			}
		}
		for _, text := range texts {
			for _, match := range channel.FindAllStringSubmatch(text, -1) {
				stamp := timestamp.FindStringSubmatch(match[1])
				if stamp == nil {
					return fmt.Errorf("%s:%d: channel message has no timestamp", args[1], index+1)
				}
				messages = append(messages, conversationMessage{stamp[1], "USER", strings.TrimSpace(match[2])})
			}
		}
	}
	sort.SliceStable(messages, func(i, j int) bool { return messages[i].Timestamp < messages[j].Timestamp })
	seen := map[conversationMessage]bool{}
	for _, message := range messages {
		if message.Text == "" || seen[message] {
			continue
		}
		seen[message] = true
		stamp, err := time.Parse(time.RFC3339Nano, message.Timestamp)
		if err != nil {
			return fmt.Errorf("%s: invalid timestamp %q: %w", args[1], message.Timestamp, err)
		}
		stamp = stamp.In(time.Local)
		day := stamp.Format("2006-01-02")
		if _, ok := selected[day]; ok {
			selected[day] = append(selected[day], fmt.Sprintf("[%s] %s: %s", stamp.Format("15:04"), message.Speaker, message.Text))
		}
	}
	heading := regexp.MustCompile(`(?m)^\[\d{2}:\d{2}\] (?:USER|ORCHESTRATOR): `)
	for day, entries := range selected {
		path := filepath.Join(directory, day+".md")
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		positions := heading.FindAllIndex(raw, 2)
		if len(positions) == 0 {
			return fmt.Errorf("%s: cannot identify existing first message", path)
		}
		end := len(raw)
		if len(positions) > 1 {
			end = positions[1][0]
		}
		first := strings.TrimSpace(string(raw[positions[0][0]:end]))
		covered := false
		for _, entry := range entries {
			if first == entry {
				covered = true
				break
			}
		}
		if !covered {
			return fmt.Errorf("%s: transcript does not cover existing first message; pass the full session transcript", path)
		}
	}
	written := map[string]bool{}
	for _, day := range dates {
		entries := selected[day]
		if len(entries) == 0 || written[day] {
			continue
		}
		written[day] = true
		header := fmt.Sprintf("# Conversation %s (user ↔ orchestrator, Telegram and terminal, verbatim; local time)\n\nEvery role reads this to check the orchestrator's words against the user's. USER lines are the user's exact\nmessages; ORCHESTRATOR lines are the orchestrator's exact replies. Nothing here is edited.\n\n", day)
		path := filepath.Join(directory, day+".md")
		if err := writeFile(path, []byte(header+strings.Join(entries, "\n\n")+"\n")); err != nil {
			return err
		}
		fmt.Printf("%s: %d messages\n", path, len(entries))
	}
	return nil
}
