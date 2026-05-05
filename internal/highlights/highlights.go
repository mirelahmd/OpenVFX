package highlights

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

type Options struct {
	MinDurationSeconds float64
	MaxDurationSeconds float64
	TopK               int
}

type Summary struct {
	ArtifactPath string
	Count        int
	TopScore     *float64
	TopStart     *float64
	TopEnd       *float64
}

type chunksDocument struct {
	Chunks []chunk `json:"chunks"`
}

type chunk struct {
	ID              string  `json:"id"`
	Start           float64 `json:"start"`
	End             float64 `json:"end"`
	DurationSeconds float64 `json:"duration_seconds"`
	Text            string  `json:"text"`
	WordCount       int     `json:"word_count"`
}

type Document struct {
	SchemaVersion string      `json:"schema_version"`
	Source        Source      `json:"source"`
	Scoring       Scoring     `json:"scoring"`
	Highlights    []Highlight `json:"highlights"`
}

type Source struct {
	ChunksArtifact string `json:"chunks_artifact"`
	Mode           string `json:"mode"`
	Strategy       string `json:"strategy"`
}

type Scoring struct {
	MinDurationSeconds float64 `json:"min_duration_seconds"`
	MaxDurationSeconds float64 `json:"max_duration_seconds"`
	TopK               int     `json:"top_k"`
}

type Highlight struct {
	ID              string  `json:"id"`
	ChunkID         string  `json:"chunk_id"`
	Start           float64 `json:"start"`
	End             float64 `json:"end"`
	DurationSeconds float64 `json:"duration_seconds"`
	Score           float64 `json:"score"`
	Label           string  `json:"label"`
	Reason          string  `json:"reason"`
	Text            string  `json:"text"`
	Signals         Signals `json:"signals"`
}

type Signals struct {
	WordCount         int  `json:"word_count"`
	HasQuestion       bool `json:"has_question"`
	HasHookPhrase     bool `json:"has_hook_phrase"`
	HasEmphasisMarker bool `json:"has_emphasis_marker"`
}

var hookPhrases = []string{
	"here's why",
	"the problem is",
	"the key is",
	"what matters",
	"the truth is",
	"this is important",
	"let me explain",
	"the mistake",
	"the reason",
}

var emphasisMarkers = []string{
	"really",
	"actually",
	"important",
	"crazy",
	"wild",
	"never",
	"always",
	"must",
	"need",
}

func WriteFromChunks(chunksPath string, outputPath string, opts Options) (Summary, error) {
	data, err := os.ReadFile(chunksPath)
	if err != nil {
		return Summary{}, fmt.Errorf("read chunks for highlights: %w", err)
	}
	var chunks chunksDocument
	if err := json.Unmarshal(data, &chunks); err != nil {
		return Summary{}, fmt.Errorf("decode chunks for highlights: %w", err)
	}

	highlights := scoreChunks(chunks.Chunks, opts)
	if len(highlights) > opts.TopK {
		highlights = highlights[:opts.TopK]
	}
	for index := range highlights {
		highlights[index].ID = fmt.Sprintf("hl_%04d", index+1)
	}

	doc := Document{
		SchemaVersion: "highlights.v1",
		Source: Source{
			ChunksArtifact: "chunks.json",
			Mode:           "deterministic",
			Strategy:       "heuristic_v1",
		},
		Scoring: Scoring{
			MinDurationSeconds: opts.MinDurationSeconds,
			MaxDurationSeconds: opts.MaxDurationSeconds,
			TopK:               opts.TopK,
		},
		Highlights: highlights,
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return Summary{}, fmt.Errorf("encode highlights: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(outputPath, out, 0o644); err != nil {
		return Summary{}, fmt.Errorf("write highlights: %w", err)
	}

	summary := Summary{ArtifactPath: "highlights.json", Count: len(highlights)}
	if len(highlights) > 0 {
		score := highlights[0].Score
		start := highlights[0].Start
		end := highlights[0].End
		summary.TopScore = &score
		summary.TopStart = &start
		summary.TopEnd = &end
	}
	return summary, nil
}

func scoreChunks(chunks []chunk, opts Options) []Highlight {
	out := make([]Highlight, 0, len(chunks))
	for _, chunk := range chunks {
		signals := Signals{
			WordCount:         chunk.WordCount,
			HasQuestion:       strings.Contains(chunk.Text, "?"),
			HasHookPhrase:     containsAny(strings.ToLower(chunk.Text), hookPhrases),
			HasEmphasisMarker: containsAnyWord(strings.ToLower(chunk.Text), emphasisMarkers),
		}
		score := scoreChunk(chunk, signals, opts)
		out = append(out, Highlight{
			ChunkID:         chunk.ID,
			Start:           chunk.Start,
			End:             chunk.End,
			DurationSeconds: chunk.DurationSeconds,
			Score:           score,
			Label:           "Candidate highlight",
			Reason:          reason(signals),
			Text:            chunk.Text,
			Signals:         signals,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})
	return out
}

func scoreChunk(chunk chunk, signals Signals, opts Options) float64 {
	if strings.TrimSpace(chunk.Text) == "" || chunk.WordCount <= 1 {
		return 0
	}
	score := 0.2
	if chunk.DurationSeconds >= opts.MinDurationSeconds && chunk.DurationSeconds <= opts.MaxDurationSeconds {
		score += 0.25
	} else {
		score -= 0.15
	}
	if chunk.WordCount >= 8 {
		score += 0.2
	} else if chunk.WordCount >= 3 {
		score += 0.08
	} else {
		score -= 0.1
	}
	if signals.HasHookPhrase {
		score += 0.2
	}
	if signals.HasQuestion {
		score += 0.12
	}
	if signals.HasEmphasisMarker {
		score += 0.13
	}
	return math.Max(0, math.Min(1, score))
}

func reason(signals Signals) string {
	var parts []string
	if signals.WordCount > 0 {
		parts = append(parts, "sufficient word count")
	}
	if signals.HasQuestion {
		parts = append(parts, "contains a question")
	}
	if signals.HasHookPhrase {
		parts = append(parts, "contains a hook phrase")
	}
	if signals.HasEmphasisMarker {
		parts = append(parts, "contains emphasis markers")
	}
	if len(parts) == 0 {
		return "Candidate selected by deterministic duration and text heuristics."
	}
	return "Candidate selected because it has " + strings.Join(parts, ", ") + "."
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func containsAnyWord(text string, words []string) bool {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	seen := map[string]bool{}
	for _, field := range fields {
		seen[field] = true
	}
	for _, word := range words {
		if seen[word] {
			return true
		}
	}
	return false
}
