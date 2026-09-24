package production

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Observation is what OpenVFX learned about the input assets before planning.
// It is produced by real ffprobe calls, never by guessing from file extensions.
type Observation struct {
	SchemaVersion string     `json:"schema_version"`
	ObservedAt    time.Time  `json:"observed_at"`
	InputPath     string     `json:"input_path"`
	Assets        []Asset    `json:"assets"`
	Skipped       []SkipNote `json:"skipped,omitempty"`
	Totals        Totals     `json:"totals"`
}

type Asset struct {
	ID              string  `json:"id"`
	Path            string  `json:"path"`
	SizeBytes       int64   `json:"size_bytes"`
	DurationSeconds float64 `json:"duration_seconds"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	FrameRate       string  `json:"frame_rate,omitempty"`
	VideoCodec      string  `json:"video_codec,omitempty"`
	AudioCodec      string  `json:"audio_codec,omitempty"`
	HasVideo        bool    `json:"has_video"`
	HasAudio        bool    `json:"has_audio"`
}

type SkipNote struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type Totals struct {
	AssetCount      int     `json:"asset_count"`
	DurationSeconds float64 `json:"duration_seconds"`
	WithAudio       int     `json:"with_audio"`
	WithVideo       int     `json:"with_video"`
}

// ProbeFunc runs ffprobe against one file and returns its raw JSON. It matches
// media.Probe so the real implementation is a direct handoff, and tests can
// substitute a fixture.
type ProbeFunc func(path string) ([]byte, error)

var mediaExtensions = map[string]bool{
	".mp4": true, ".mov": true, ".m4v": true, ".mkv": true, ".avi": true,
	".webm": true, ".mxf": true, ".wav": true, ".mp3": true, ".m4a": true,
	".aac": true, ".flac": true,
}

// Observe walks inputPath (a file or a directory) and probes every media file
// it finds. Non-media files are recorded as skipped with a reason rather than
// silently dropped — a production that ignored an asset should say so.
func Observe(inputPath string, probe ProbeFunc) (Observation, error) {
	abs, err := filepath.Abs(inputPath)
	if err != nil {
		return Observation{}, fmt.Errorf("resolve input path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Observation{}, fmt.Errorf("input path not found: %s", abs)
	}

	var candidates []string
	if info.IsDir() {
		entries, err := os.ReadDir(abs)
		if err != nil {
			return Observation{}, fmt.Errorf("read input directory: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			candidates = append(candidates, filepath.Join(abs, e.Name()))
		}
		sort.Strings(candidates)
	} else {
		candidates = []string{abs}
	}

	obs := Observation{
		SchemaVersion: ObservationSchemaVersion,
		ObservedAt:    time.Now().UTC(),
		InputPath:     abs,
	}

	for _, path := range candidates {
		ext := strings.ToLower(filepath.Ext(path))
		if !mediaExtensions[ext] {
			obs.Skipped = append(obs.Skipped, SkipNote{Path: path, Reason: "unrecognised media extension " + ext})
			continue
		}
		raw, err := probe(path)
		if err != nil {
			obs.Skipped = append(obs.Skipped, SkipNote{Path: path, Reason: "ffprobe failed: " + err.Error()})
			continue
		}
		asset, err := parseProbe(path, raw)
		if err != nil {
			obs.Skipped = append(obs.Skipped, SkipNote{Path: path, Reason: err.Error()})
			continue
		}
		if st, err := os.Stat(path); err == nil {
			asset.SizeBytes = st.Size()
		}
		asset.ID = fmt.Sprintf("asset_%04d", len(obs.Assets)+1)
		obs.Assets = append(obs.Assets, asset)
	}

	if len(obs.Assets) == 0 {
		return obs, fmt.Errorf("no probeable media found at %s", abs)
	}

	for _, a := range obs.Assets {
		obs.Totals.DurationSeconds += a.DurationSeconds
		if a.HasAudio {
			obs.Totals.WithAudio++
		}
		if a.HasVideo {
			obs.Totals.WithVideo++
		}
	}
	obs.Totals.AssetCount = len(obs.Assets)
	obs.Totals.DurationSeconds = round3(obs.Totals.DurationSeconds)
	return obs, nil
}

// VideoAssets returns only the assets carrying a video stream, in order.
func (o Observation) VideoAssets() []Asset {
	var out []Asset
	for _, a := range o.Assets {
		if a.HasVideo {
			out = append(out, a)
		}
	}
	return out
}

// FirstWithAudio returns the first asset carrying an audio stream.
func (o Observation) FirstWithAudio() (Asset, bool) {
	for _, a := range o.Assets {
		if a.HasAudio {
			return a, true
		}
	}
	return Asset{}, false
}

type ffprobeDoc struct {
	Format *struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		RFrameRate   string `json:"r_frame_rate"`
		AvgFrameRate string `json:"avg_frame_rate"`
		Duration     string `json:"duration"`
	} `json:"streams"`
}

func parseProbe(path string, raw []byte) (Asset, error) {
	var doc ffprobeDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Asset{}, fmt.Errorf("ffprobe output is not valid JSON: %w", err)
	}
	asset := Asset{Path: path}
	if doc.Format != nil {
		asset.DurationSeconds = round3(parseFloat(doc.Format.Duration))
	}
	for _, s := range doc.Streams {
		switch s.CodecType {
		case "video":
			if !asset.HasVideo {
				asset.HasVideo = true
				asset.Width = s.Width
				asset.Height = s.Height
				asset.VideoCodec = s.CodecName
				asset.FrameRate = pickFrameRate(s.RFrameRate, s.AvgFrameRate)
			}
		case "audio":
			if !asset.HasAudio {
				asset.HasAudio = true
				asset.AudioCodec = s.CodecName
			}
		}
		// Some containers carry duration only on the stream.
		if asset.DurationSeconds == 0 {
			asset.DurationSeconds = round3(parseFloat(s.Duration))
		}
	}
	if !asset.HasVideo && !asset.HasAudio {
		return Asset{}, fmt.Errorf("no video or audio stream found")
	}
	return asset, nil
}

func pickFrameRate(candidates ...string) string {
	for _, c := range candidates {
		if c != "" && c != "0/0" {
			return c
		}
	}
	return ""
}

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

func round3(v float64) float64 {
	return float64(int64(v*1000+0.5)) / 1000
}
