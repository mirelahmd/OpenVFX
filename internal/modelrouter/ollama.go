package modelrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultOllamaBaseURL = "http://localhost:11434"

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

var newHTTPClient = func(timeout time.Duration) HTTPDoer {
	return &http.Client{Timeout: timeout}
}

type OllamaAdapter struct {
	Timeout time.Duration
}

func NewOllamaAdapter() Adapter {
	return OllamaAdapter{Timeout: 120 * time.Second}
}

func (a OllamaAdapter) Name() string {
	return "ollama"
}

func (a OllamaAdapter) Supports(provider string) bool {
	switch provider {
	case "ollama", "ollama-local":
		return true
	default:
		return false
	}
}

func (a OllamaAdapter) BuildRequest(req Request) (Request, error) {
	if strings.TrimSpace(req.Model) == "" {
		return req, fmt.Errorf("ollama model is required")
	}
	req.RequestPreview = buildPreview(req)
	if strings.TrimSpace(req.BaseURL) == "" {
		req.BaseURL = defaultOllamaBaseURL
	}
	req.Status = "ollama_ready"
	return req, nil
}

func (a OllamaAdapter) Execute(req Request) (Response, error) {
	built, err := a.BuildRequest(req)
	if err != nil {
		return Response{}, err
	}
	prompt := strings.TrimSpace(built.RequestPreview.System + "\n\n" + built.RequestPreview.User)
	body := map[string]any{
		"model":  built.Model,
		"prompt": prompt,
		"stream": false,
	}
	if len(built.Options) > 0 {
		body["options"] = built.Options
	}
	data, err := json.Marshal(body)
	if err != nil {
		return Response{}, fmt.Errorf("encode ollama request: %w", err)
	}

	client := newHTTPClient(a.timeout())
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout())
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(built.BaseURL, "/")+"/api/generate", bytes.NewReader(data))
	if err != nil {
		return Response{}, fmt.Errorf("build ollama request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("Ollama request failed. Is Ollama running at %s?", built.BaseURL)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read ollama response: %w", err)
	}

	var payload struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(respData, &payload); err != nil {
		if resp.StatusCode >= 400 {
			return Response{}, fmt.Errorf("ollama request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(respData)))
		}
		return Response{}, fmt.Errorf("decode ollama response: %w", err)
	}
	if resp.StatusCode >= 400 {
		if strings.TrimSpace(payload.Error) != "" {
			return Response{}, fmt.Errorf("ollama request failed: %s", payload.Error)
		}
		return Response{}, fmt.Errorf("ollama request failed with status %d", resp.StatusCode)
	}
	if strings.TrimSpace(payload.Error) != "" {
		return Response{}, fmt.Errorf("ollama request failed: %s", payload.Error)
	}

	texts, mode, warnings := parseOllamaResponseTexts(req.TaskType, payload.Response)
	return Response{
		Status:   "completed",
		Texts:    texts,
		Mode:     mode,
		Warnings: warnings,
		Details: map[string]any{
			"adapter": "ollama",
		},
	}, nil
}

func (a OllamaAdapter) timeout() time.Duration {
	if a.Timeout <= 0 {
		return 120 * time.Second
	}
	return a.Timeout
}

func CheckOllama(baseURL string, timeout time.Duration) error {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultOllamaBaseURL
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := newHTTPClient(timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("build ollama request: %w", err)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("Ollama request failed. Is Ollama running at %s?", baseURL)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func SetHTTPClientFactoryForTests(factory func(timeout time.Duration) HTTPDoer) func() {
	previous := newHTTPClient
	newHTTPClient = factory
	return func() {
		newHTTPClient = previous
	}
}

func parseOllamaResponseTexts(taskType string, response string) ([]string, string, []string) {
	response = strings.TrimSpace(response)
	if response == "" {
		return []string{""}, "plain_text", []string{"provider returned empty response"}
	}

	var object map[string]any
	if err := json.Unmarshal([]byte(response), &object); err != nil {
		return []string{response}, "plain_text", nil
	}

	var texts []string
	switch taskType {
	case "caption_variants":
		texts = extractTextsFromObject(object, "items", "captions")
	case "timeline_labels":
		texts = extractTextsFromObject(object, "items", "labels")
	case "short_descriptions":
		texts = extractTextsFromObject(object, "items", "descriptions")
	default:
		texts = extractTextsFromObject(object, "items", "captions", "labels", "descriptions")
	}
	if len(texts) == 0 {
		return []string{response}, "json_fallback", []string{"provider returned JSON that did not match the expected shape; using plain text fallback"}
	}
	return texts, "json", nil
}

func extractTextsFromObject(object map[string]any, keys ...string) []string {
	var out []string
	for _, key := range keys {
		raw, ok := object[key]
		if !ok {
			continue
		}
		switch value := raw.(type) {
		case []any:
			for _, item := range value {
				switch typed := item.(type) {
				case string:
					typed = strings.TrimSpace(typed)
					if typed != "" {
						out = append(out, typed)
					}
				case map[string]any:
					if text, ok := typed["text"].(string); ok {
						text = strings.TrimSpace(text)
						if text != "" {
							out = append(out, text)
						}
					}
				}
			}
		}
	}
	return out
}
