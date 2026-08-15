package langfuse

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PromptRecord is a registered, system-owned static prompt. Category is
// local metadata used by the CLI; Langfuse receives name/type/content/tags.
type PromptRecord struct {
	Name     string
	Type     string
	Content  string
	Labels   []string
	Tags     []string
	Category string
	Version  int
}

type PromptSyncClient struct {
	cfg    Config
	client *http.Client
}

func NewPromptSyncClient(cfg Config) *PromptSyncClient {
	return &PromptSyncClient{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}}
}

func (c *PromptSyncClient) authHeader() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(c.cfg.PublicKey+":"+c.cfg.SecretKey))
}

func (c *PromptSyncClient) SyncPrompt(ctx context.Context, prompt PromptRecord) error {
	if !c.cfg.HasCredentials() {
		return fmt.Errorf("langfuse prompt sync requires public and secret keys")
	}
	if prompt.Type == "" {
		prompt.Type = "text"
	}
	if len(prompt.Labels) == 0 {
		prompt.Labels = []string{"production"}
	}
	body, err := json.Marshal(struct {
		Name     string   `json:"name"`
		Type     string   `json:"type"`
		Prompt   string   `json:"prompt"`
		Labels   []string `json:"labels,omitempty"`
		Tags     []string `json:"tags,omitempty"`
		Version  int      `json:"version,omitempty"`
		IsActive bool     `json:"isActive"`
	}{prompt.Name, prompt.Type, prompt.Content, prompt.Labels, prompt.Tags, prompt.Version, true})
	if err != nil {
		return fmt.Errorf("marshal prompt: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.Host, "/")+"/api/public/v2/prompts", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("User-Agent", "weknora-langfuse-prompt-sync/1.0")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("langfuse prompt sync: status %d: %s", resp.StatusCode, string(data))
	}
	return nil
}

// FetchPrompt is intentionally read-only. The reference mechanism exposes it
// for inspection; it does not rewrite local Go constants or YAML templates.
func (c *PromptSyncClient) FetchPrompt(ctx context.Context, name, label string) (*PromptRecord, error) {
	if !c.cfg.HasCredentials() {
		return nil, fmt.Errorf("langfuse prompt sync requires public and secret keys")
	}
	if label == "" {
		label = "production"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/public/v2/prompts/%s?label=%s", strings.TrimRight(c.cfg.Host, "/"), name, label), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("langfuse prompt fetch: status %d: %s", resp.StatusCode, string(data))
	}
	var raw struct {
		Name    string   `json:"name"`
		Type    string   `json:"type"`
		Prompt  string   `json:"prompt"`
		Labels  []string `json:"labels"`
		Tags    []string `json:"tags"`
		Version int      `json:"version"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return &PromptRecord{Name: raw.Name, Type: raw.Type, Content: raw.Prompt, Labels: raw.Labels, Tags: raw.Tags, Version: raw.Version}, nil
}
