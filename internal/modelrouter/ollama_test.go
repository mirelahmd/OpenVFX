package modelrouter

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestOllamaAdapterSupportsProviderStrings(t *testing.T) {
	adapter := NewOllamaAdapter()
	if !adapter.Supports("ollama") {
		t.Fatal("expected support for ollama")
	}
	if !adapter.Supports("ollama-local") {
		t.Fatal("expected support for ollama-local")
	}
	if adapter.Supports("openai") {
		t.Fatal("unexpected support for openai")
	}
}

func TestOllamaAdapterBuildsRequestWithDefaultBaseURL(t *testing.T) {
	adapter := NewOllamaAdapter()
	req, err := adapter.BuildRequest(Request{
		TaskID:    "task_0001",
		TaskType:  "caption_variants",
		RouteName: "caption_expansion",
		Provider:  "ollama",
		Model:     "qwen2.5:7b",
		Input:     RequestInput{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.BaseURL != defaultOllamaBaseURL {
		t.Fatalf("base_url = %q", req.BaseURL)
	}
	if req.RequestPreview.User == "" {
		t.Fatal("expected request preview")
	}
}

func TestCheckOllamaMissingReturnsCleanError(t *testing.T) {
	restore := SetHTTPClientFactoryForTests(func(timeout time.Duration) HTTPDoer {
		return roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		})
	})
	defer restore()

	err := CheckOllama("http://localhost:11434", time.Second)
	if err == nil || err.Error() != "Ollama request failed. Is Ollama running at http://localhost:11434?" {
		t.Fatalf("err = %v", err)
	}
}

func TestOllamaAdapterExecuteParsesResponse(t *testing.T) {
	restore := SetHTTPClientFactoryForTests(func(timeout time.Duration) HTTPDoer {
		return roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"response":"hello world"}`)),
				Header:     make(http.Header),
			}, nil
		})
	})
	defer restore()

	adapter := NewOllamaAdapter()
	resp, err := adapter.Execute(Request{
		TaskID:    "task_0001",
		TaskType:  "caption_variants",
		RouteName: "caption_expansion",
		Provider:  "ollama",
		Model:     "qwen2.5:7b",
		BaseURL:   "http://localhost:11434",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Texts) != 1 || resp.Texts[0] != "hello world" {
		t.Fatalf("texts = %#v", resp.Texts)
	}
}

func TestParseOllamaResponseTextsItems(t *testing.T) {
	texts, mode, warnings := parseOllamaResponseTexts("caption_variants", `{"items":[{"text":"a"},{"text":"b"}]}`)
	if mode != "json" || len(warnings) != 0 || len(texts) != 2 {
		t.Fatalf("texts=%#v mode=%s warnings=%#v", texts, mode, warnings)
	}
}

func TestParseOllamaResponseTextsCaptionsLabelsDescriptions(t *testing.T) {
	captions, mode, _ := parseOllamaResponseTexts("caption_variants", `{"captions":["a","b"]}`)
	if mode != "json" || len(captions) != 2 {
		t.Fatalf("captions=%#v mode=%s", captions, mode)
	}
	labels, mode, _ := parseOllamaResponseTexts("timeline_labels", `{"labels":["x"]}`)
	if mode != "json" || len(labels) != 1 || labels[0] != "x" {
		t.Fatalf("labels=%#v mode=%s", labels, mode)
	}
	descriptions, mode, _ := parseOllamaResponseTexts("short_descriptions", `{"descriptions":["d"]}`)
	if mode != "json" || len(descriptions) != 1 || descriptions[0] != "d" {
		t.Fatalf("descriptions=%#v mode=%s", descriptions, mode)
	}
}

func TestParseOllamaResponseTextsPlainTextAndFallback(t *testing.T) {
	texts, mode, warnings := parseOllamaResponseTexts("caption_variants", `plain text`)
	if mode != "plain_text" || len(texts) != 1 || texts[0] != "plain text" {
		t.Fatalf("texts=%#v mode=%s", texts, mode)
	}
	texts, mode, warnings = parseOllamaResponseTexts("caption_variants", `{"unexpected":["x"]}`)
	if mode != "json_fallback" || len(texts) != 1 || len(warnings) == 0 {
		t.Fatalf("texts=%#v mode=%s warnings=%#v", texts, mode, warnings)
	}
}
