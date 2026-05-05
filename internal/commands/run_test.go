package commands

import (
	"strings"
	"testing"
)

func TestSummarizeMetadataCountsStreams(t *testing.T) {
	raw := []byte(`{
	  "streams": [
	    {"codec_type": "video"},
	    {"codec_type": "audio"},
	    {"codec_type": "subtitle"}
	  ],
	  "format": {"duration": "2.000000"}
	}`)

	summary := summarizeMetadata(raw)

	if summary.Duration != "2.000000" {
		t.Fatalf("Duration = %q, want 2.000000", summary.Duration)
	}
	if summary.VideoStreams == nil || *summary.VideoStreams != 1 {
		t.Fatalf("VideoStreams = %v, want 1", summary.VideoStreams)
	}
	if summary.AudioStreams == nil || *summary.AudioStreams != 1 {
		t.Fatalf("AudioStreams = %v, want 1", summary.AudioStreams)
	}
	if summary.TotalStreams == nil || *summary.TotalStreams != 3 {
		t.Fatalf("TotalStreams = %v, want 3", summary.TotalStreams)
	}
}

func TestSummarizeMetadataUnknownWhenJSONInvalid(t *testing.T) {
	summary := summarizeMetadata([]byte(`not json`))

	if durationDisplay(summary.Duration) != "unknown" {
		t.Fatalf("durationDisplay = %q, want unknown", durationDisplay(summary.Duration))
	}
	if countDisplay(summary.VideoStreams) != "unknown" {
		t.Fatalf("video countDisplay = %q, want unknown", countDisplay(summary.VideoStreams))
	}
}

func TestSummaryDisplayUsesUnknownForMissingFields(t *testing.T) {
	raw := []byte(`{}`)

	summary := summarizeMetadata(raw)

	if durationDisplay(summary.Duration) != "unknown" {
		t.Fatalf("durationDisplay = %q, want unknown", durationDisplay(summary.Duration))
	}
	if got := countDisplay(summary.VideoStreams); got != "unknown" {
		t.Fatalf("video countDisplay = %q, want unknown", got)
	}
	if got := countDisplay(summary.AudioStreams); got != "unknown" {
		t.Fatalf("audio countDisplay = %q, want unknown", got)
	}
	if got := countDisplay(summary.TotalStreams); got != "unknown" {
		t.Fatalf("total countDisplay = %q, want unknown", got)
	}
}

func TestSummaryDisplayUsesZeroForExplicitEmptyStreamList(t *testing.T) {
	raw := []byte(`{"streams":[]}`)

	summary := summarizeMetadata(raw)

	if got := countDisplay(summary.VideoStreams); got != "0" {
		t.Fatalf("video countDisplay = %q, want 0", got)
	}
	if got := countDisplay(summary.AudioStreams); got != "0" {
		t.Fatalf("audio countDisplay = %q, want 0", got)
	}
	if got := countDisplay(summary.TotalStreams); got != "0" {
		t.Fatalf("total countDisplay = %q, want 0", got)
	}
}

func TestDurationDisplayAppendsSeconds(t *testing.T) {
	if got := durationDisplay("1.500000"); !strings.Contains(got, "seconds") {
		t.Fatalf("durationDisplay = %q, want seconds suffix", got)
	}
}
