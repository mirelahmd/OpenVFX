package chunks

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Options struct {
	TargetSeconds float64
	MaxGapSeconds float64
}

type Summary struct {
	ArtifactPath  string
	ChunkCount    int
	TargetSeconds float64
	MaxGapSeconds float64
}

type transcriptDocument struct {
	Segments []transcriptSegment `json:"segments"`
}

type transcriptSegment struct {
	ID    string  `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type Document struct {
	SchemaVersion string   `json:"schema_version"`
	Source        Source   `json:"source"`
	Chunking      Chunking `json:"chunking"`
	Chunks        []Chunk  `json:"chunks"`
}

type Source struct {
	TranscriptArtifact string `json:"transcript_artifact"`
	Mode               string `json:"mode"`
	Strategy           string `json:"strategy"`
}

type Chunking struct {
	TargetSeconds float64 `json:"target_seconds"`
	MaxGapSeconds float64 `json:"max_gap_seconds"`
}

type Chunk struct {
	ID              string   `json:"id"`
	Start           float64  `json:"start"`
	End             float64  `json:"end"`
	DurationSeconds float64  `json:"duration_seconds"`
	Text            string   `json:"text"`
	SegmentIDs      []string `json:"segment_ids"`
	WordCount       int      `json:"word_count"`
}

func WriteFromTranscript(transcriptPath string, outputPath string, opts Options) (Summary, error) {
	data, err := os.ReadFile(transcriptPath)
	if err != nil {
		return Summary{}, fmt.Errorf("read transcript for chunking: %w", err)
	}

	var transcript transcriptDocument
	if err := json.Unmarshal(data, &transcript); err != nil {
		return Summary{}, fmt.Errorf("decode transcript for chunking: %w", err)
	}

	doc := Document{
		SchemaVersion: "chunks.v1",
		Source: Source{
			TranscriptArtifact: "transcript.json",
			Mode:               "deterministic",
			Strategy:           "time_window_v1",
		},
		Chunking: Chunking{
			TargetSeconds: opts.TargetSeconds,
			MaxGapSeconds: opts.MaxGapSeconds,
		},
		Chunks: buildChunks(transcript.Segments, opts),
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return Summary{}, fmt.Errorf("encode chunks: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(outputPath, out, 0o644); err != nil {
		return Summary{}, fmt.Errorf("write chunks: %w", err)
	}

	return Summary{
		ArtifactPath:  "chunks.json",
		ChunkCount:    len(doc.Chunks),
		TargetSeconds: opts.TargetSeconds,
		MaxGapSeconds: opts.MaxGapSeconds,
	}, nil
}

func buildChunks(segments []transcriptSegment, opts Options) []Chunk {
	var out []Chunk
	var current []transcriptSegment

	flush := func() {
		if len(current) == 0 {
			return
		}
		out = append(out, makeChunk(len(out)+1, current))
		current = nil
	}

	for _, segment := range segments {
		if len(current) == 0 {
			current = append(current, segment)
			continue
		}

		previous := current[len(current)-1]
		gap := segment.Start - previous.End
		wouldExceedTarget := segment.End-current[0].Start > opts.TargetSeconds
		if gap > opts.MaxGapSeconds || wouldExceedTarget {
			flush()
		}
		current = append(current, segment)
	}
	flush()

	return out
}

func makeChunk(index int, segments []transcriptSegment) Chunk {
	start := segments[0].Start
	end := segments[len(segments)-1].End
	var textParts []string
	var segmentIDs []string
	for _, segment := range segments {
		textParts = append(textParts, strings.TrimSpace(segment.Text))
		segmentIDs = append(segmentIDs, segment.ID)
	}
	text := strings.Join(nonEmpty(textParts), " ")
	return Chunk{
		ID:              fmt.Sprintf("chunk_%04d", index),
		Start:           start,
		End:             end,
		DurationSeconds: end - start,
		Text:            text,
		SegmentIDs:      segmentIDs,
		WordCount:       len(strings.Fields(text)),
	}
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
