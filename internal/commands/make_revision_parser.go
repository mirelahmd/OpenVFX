package commands

import (
	"fmt"
	"strings"
)

// ---- action type constants ----

const (
	RevisionActionSetPlatform        = "set_platform"
	RevisionActionSetCaptionPosition = "set_caption_position"
	RevisionActionSetCaptionStyle    = "set_caption_style"
	RevisionActionGenerateScript     = "generate_script"
	RevisionActionGenerateCaptions   = "generate_captions"
	RevisionActionPrepareVoiceover   = "prepare_voiceover"
	RevisionActionGenerateVoiceover  = "generate_voiceover"
	RevisionActionMixVoiceover       = "mix_voiceover"
	RevisionActionReassemble         = "reassemble"
	RevisionActionValidate           = "validate"
)

// ParsedRevisionAction is the output of the deterministic parser.
type ParsedRevisionAction struct {
	Type             string            `json:"type"`
	Description      string            `json:"description"`
	Params           map[string]string `json:"params,omitempty"`
	RequiresProvider bool              `json:"requires_provider"`
}

var revisionExamples = `
Supported requests:
  Platform:   "switch to tiktok", "instagram reel", "square", "youtube short", "vertical"
  Captions:   "move captions to center", "captions top", "move captions to bottom"
  Style:      "boxed captions", "make captions bold", "default captions"
  Duration:   "make it shorter", "make it longer"
  Script:     "regenerate script", "rewrite script", "more cinematic script"
  Captions:   "regenerate captions", "new captions", "more caption options"
  Voiceover:  "prepare voiceover", "voiceover text", "generate voiceover", "mix voiceover"
  Reassemble: "reassemble", "render again", "make new draft"`

// ParseRevisionRequest maps a natural-language revision request to a list of planned actions.
// The parser is deterministic and local-first — no LLM is involved.
// Returns an error with examples when the request cannot be mapped.
func ParseRevisionRequest(request string) ([]ParsedRevisionAction, error) {
	lower := strings.ToLower(strings.TrimSpace(request))
	if lower == "" {
		return nil, fmt.Errorf("revision request is empty; provide a --request value\n%s", revisionExamples)
	}

	// Platform
	if platform, ok := matchPlatformRequest(lower); ok {
		return []ParsedRevisionAction{{
			Type:        RevisionActionSetPlatform,
			Description: fmt.Sprintf("Switch platform to %s and reassemble", platform),
			Params:      map[string]string{"platform": platform},
		}}, nil
	}

	// Caption position — check before style (to avoid "center" matching style)
	if position, ok := matchCaptionPositionRequest(lower); ok {
		return []ParsedRevisionAction{{
			Type:        RevisionActionSetCaptionPosition,
			Description: fmt.Sprintf("Set caption position to %s", position),
			Params:      map[string]string{"position": position},
		}}, nil
	}

	// Caption style
	if style, ok := matchCaptionStyleRequest(lower); ok {
		return []ParsedRevisionAction{{
			Type:        RevisionActionSetCaptionStyle,
			Description: fmt.Sprintf("Set caption style to %s", style),
			Params:      map[string]string{"style": style},
		}}, nil
	}

	// Duration — shorter
	if matchShorterRequest(lower) {
		return []ParsedRevisionAction{{
			Type:        RevisionActionReassemble,
			Description: "Make shorter: reassemble with current clip selection (v1: selection unchanged; manually re-run roughcut for fewer clips)",
			Params:      map[string]string{"duration_hint": "shorter"},
		}}, nil
	}

	// Duration — longer
	if matchLongerRequest(lower) {
		return []ParsedRevisionAction{{
			Type:        RevisionActionReassemble,
			Description: "Make longer: reassemble with current clip selection (v1: selection unchanged; manually add clips first)",
			Params:      map[string]string{"duration_hint": "longer"},
		}}, nil
	}

	// Script regeneration (with optional tone)
	if tone, ok := matchScriptRequest(lower); ok {
		params := map[string]string{}
		if tone != "" {
			params["tone"] = tone
		}
		desc := "Regenerate script"
		if tone != "" {
			desc += " with tone: " + tone
		}
		return []ParsedRevisionAction{{
			Type:             RevisionActionGenerateScript,
			Description:      desc,
			Params:           params,
			RequiresProvider: true,
		}}, nil
	}

	// Caption variants regeneration (check before prepare_voiceover / voiceover words)
	if matchCaptionVariantsRequest(lower) {
		return []ParsedRevisionAction{{
			Type:             RevisionActionGenerateCaptions,
			Description:      "Regenerate caption variants",
			RequiresProvider: true,
		}}, nil
	}

	// Voiceover text preparation (no provider)
	if matchPrepareVoiceoverRequest(lower) {
		return []ParsedRevisionAction{{
			Type:        RevisionActionPrepareVoiceover,
			Description: "Prepare voiceover text from script/goal",
		}}, nil
	}

	// Voiceover audio generation (requires provider)
	if matchGenerateVoiceoverRequest(lower) {
		return []ParsedRevisionAction{{
			Type:             RevisionActionGenerateVoiceover,
			Description:      "Generate voiceover audio",
			RequiresProvider: true,
		}}, nil
	}

	// Mix voiceover (reassemble with voiceover)
	if matchMixVoiceoverRequest(lower) {
		return []ParsedRevisionAction{{
			Type:        RevisionActionMixVoiceover,
			Description: "Mix voiceover into draft (reassemble with --mix-voiceover)",
		}}, nil
	}

	// Reassemble / render again
	if matchReassembleRequest(lower) {
		return []ParsedRevisionAction{{
			Type:        RevisionActionReassemble,
			Description: "Reassemble draft with current settings",
		}}, nil
	}

	return nil, fmt.Errorf("could not map revision request to a supported deterministic action: %q\n%s", request, revisionExamples)
}

// ---- keyword matchers ----

func matchPlatformRequest(lower string) (string, bool) {
	switch {
	case containsAny(lower, "tiktok", "tik tok"):
		return "tiktok", true
	case containsAny(lower, "instagram-reel", "instagram reel", "instagram", "reel", "reels", "ig"):
		return "instagram-reel", true
	case containsAny(lower, "youtube short", "youtube-short", "yt-short", "yt short"):
		return "youtube-short", true
	case containsAny(lower, "vertical") && !containsAny(lower, "horizontal"):
		return "tiktok", true
	case containsAny(lower, "youtube", "yt") && !containsAny(lower, "short"):
		return "youtube", true
	case containsAny(lower, "square"):
		return "square", true
	}
	return "", false
}

func matchCaptionPositionRequest(lower string) (string, bool) {
	hasCaption := containsAny(lower, "caption", "captions", "subtitle", "subtitles")
	hasPositionWord := containsAny(lower, "move", "set", "put", "place", "position", "to")

	centerMatch := containsAny(lower, "center", "middle")
	topMatch := containsAny(lower, "top", "upper")
	bottomMatch := containsAny(lower, "bottom", "lower")

	// Pattern: "move captions to X" / "captions X" / "X captions"
	if (hasCaption || hasPositionWord) && centerMatch {
		return "center", true
	}
	if (hasCaption || hasPositionWord) && topMatch {
		return "top", true
	}
	if (hasCaption || hasPositionWord) && bottomMatch {
		return "bottom", true
	}

	return "", false
}

func matchCaptionStyleRequest(lower string) (string, bool) {
	hasCaption := containsAny(lower, "caption", "captions", "subtitle", "subtitles")

	switch {
	case hasCaption && containsAny(lower, "boxed", "box"):
		return "boxed", true
	case hasCaption && containsAny(lower, "bold"):
		return "bold", true
	case hasCaption && containsAny(lower, "default", "reset", "normal", "plain"):
		return "default", true
	}
	return "", false
}

func matchShorterRequest(lower string) bool {
	return containsAny(lower, "shorter", "trim", "cut down", "cut shorter", "fewer clips", "less clips")
}

func matchLongerRequest(lower string) bool {
	return containsAny(lower, "longer", "extend", "add more", "more clips", "more content")
}

func matchScriptRequest(lower string) (tone string, ok bool) {
	if !containsAny(lower, "script", "rewrite") {
		return "", false
	}

	// Extract tone from common patterns (using prefix/contains matching for inflected forms)
	for _, pair := range []struct{ keyword, tone string }{
		{"cinemati", "cinematic"}, // matches cinematic, cinematically
		{"funni", "funny"},        // matches funny, funnier, funniest
		{"humor", "funny"},
		{"profession", "professional"},
		{"casual", "casual"},
		{"energeti", "energetic"},
		{"serious", "serious"},
		{"dramati", "dramatic"},
	} {
		if strings.Contains(lower, pair.keyword) {
			return pair.tone, true
		}
	}

	return "", true
}

func matchCaptionVariantsRequest(lower string) bool {
	return containsAny(lower,
		"regenerate captions", "new captions", "more caption options",
		"caption variants", "more captions", "different captions",
		"regenerate caption variants",
	)
}

func matchPrepareVoiceoverRequest(lower string) bool {
	return containsAny(lower,
		"prepare voiceover", "voiceover text", "prepare voice over", "voice over text",
	)
}

func matchGenerateVoiceoverRequest(lower string) bool {
	return containsAny(lower,
		"generate voiceover", "regenerate voiceover", "new voiceover",
		"generate voice over", "regenerate voice over",
	)
}

func matchMixVoiceoverRequest(lower string) bool {
	return containsAny(lower,
		"mix voiceover", "mix voice over", "add voiceover", "add voice over",
	)
}

func matchReassembleRequest(lower string) bool {
	return containsAny(lower,
		"reassemble", "render again", "make new draft", "re-assemble",
		"rebuild draft", "new draft", "rerender", "re-render",
	)
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
