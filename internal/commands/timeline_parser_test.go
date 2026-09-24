package commands

import (
	"testing"
)

func TestParseTimeInstruction_WholeVideo(t *testing.T) {
	dur := 120.0
	cases := []string{
		"use the whole video",
		"use entire video",
		"use the full video",
	}
	for _, goal := range cases {
		inst := ParseTimeInstruction(goal, dur)
		if inst.Start != 0 || inst.End != dur {
			t.Errorf("goal=%q: want start=0 end=%.0f, got start=%.2f end=%.2f", goal, dur, inst.Start, inst.End)
		}
		if inst.RepeatCount != 1 {
			t.Errorf("goal=%q: want repeat=1, got %d", goal, inst.RepeatCount)
		}
	}
}

func TestParseTimeInstruction_FirstNSeconds(t *testing.T) {
	dur := 60.0
	inst := ParseTimeInstruction("use the first 10 seconds", dur)
	if inst.Start != 0 || inst.End != 10 {
		t.Errorf("want start=0 end=10, got start=%.2f end=%.2f", inst.Start, inst.End)
	}
	if inst.RepeatCount != 1 {
		t.Errorf("want repeat=1, got %d", inst.RepeatCount)
	}
}

func TestParseTimeInstruction_FirstNSeconds_Clamp(t *testing.T) {
	dur := 5.0
	inst := ParseTimeInstruction("first 30 seconds", dur)
	if inst.End != dur {
		t.Errorf("want end clamped to %.2f, got %.2f", dur, inst.End)
	}
	if len(inst.Warnings) == 0 {
		t.Error("expected clamp warning, got none")
	}
}

func TestParseTimeInstruction_LastNSeconds(t *testing.T) {
	dur := 60.0
	inst := ParseTimeInstruction("use the last 15 seconds", dur)
	if inst.Start != 45 || inst.End != 60 {
		t.Errorf("want start=45 end=60, got start=%.2f end=%.2f", inst.Start, inst.End)
	}
}

func TestParseTimeInstruction_LastNSeconds_Clamp(t *testing.T) {
	dur := 5.0
	inst := ParseTimeInstruction("last 30 seconds", dur)
	if inst.Start != 0 || inst.End != dur {
		t.Errorf("want whole video fallback (start=0 end=%.2f), got start=%.2f end=%.2f", dur, inst.Start, inst.End)
	}
	if len(inst.Warnings) == 0 {
		t.Error("expected clamp warning, got none")
	}
}

func TestParseTimeInstruction_FromTo(t *testing.T) {
	dur := 120.0
	cases := []struct {
		goal        string
		wantStart   float64
		wantEnd     float64
	}{
		{"from 10 to 30 seconds", 10, 30},
		{"use 5 to 20 seconds", 5, 20},
		{"clip from 0 to 60 seconds", 0, 60},
		{"use seconds 10 to 40", 10, 40},
	}
	for _, tc := range cases {
		inst := ParseTimeInstruction(tc.goal, dur)
		if inst.Start != tc.wantStart || inst.End != tc.wantEnd {
			t.Errorf("goal=%q: want start=%.0f end=%.0f, got start=%.2f end=%.2f",
				tc.goal, tc.wantStart, tc.wantEnd, inst.Start, inst.End)
		}
	}
}

func TestParseTimeInstruction_SecondsDash(t *testing.T) {
	dur := 120.0
	inst := ParseTimeInstruction("seconds 10-30", dur)
	if inst.Start != 10 || inst.End != 30 {
		t.Errorf("want start=10 end=30, got start=%.2f end=%.2f", inst.Start, inst.End)
	}
}

func TestParseTimeInstruction_LoopLast(t *testing.T) {
	dur := 10.0
	inst := ParseTimeInstruction("loop the last 2 seconds", dur)
	if inst.Start != 8 || inst.End != 10 {
		t.Errorf("want start=8 end=10, got start=%.2f end=%.2f", inst.Start, inst.End)
	}
	if inst.RepeatCount < 2 {
		t.Errorf("loop without count should default to 2 repeats, got %d", inst.RepeatCount)
	}
}

func TestParseTimeInstruction_LoopLastExplicitCount(t *testing.T) {
	dur := 10.0
	inst := ParseTimeInstruction("loop the last 2 seconds 3 times", dur)
	if inst.RepeatCount != 3 {
		t.Errorf("want repeat=3, got %d", inst.RepeatCount)
	}
	if inst.Start != 8 || inst.End != 10 {
		t.Errorf("want start=8 end=10, got start=%.2f end=%.2f", inst.Start, inst.End)
	}
}

func TestParseTimeInstruction_RepeatCount_Twice(t *testing.T) {
	dur := 60.0
	inst := ParseTimeInstruction("use the whole video twice", dur)
	if inst.RepeatCount != 2 {
		t.Errorf("want repeat=2, got %d", inst.RepeatCount)
	}
}

func TestParseTimeInstruction_RepeatCount_ThreeTimes(t *testing.T) {
	dur := 60.0
	inst := ParseTimeInstruction("first 10 seconds three times", dur)
	if inst.RepeatCount != 3 {
		t.Errorf("want repeat=3, got %d", inst.RepeatCount)
	}
}

func TestParseTimeInstruction_RepeatCount_NTimes(t *testing.T) {
	dur := 60.0
	inst := ParseTimeInstruction("first 10 seconds 5 times", dur)
	if inst.RepeatCount != 5 {
		t.Errorf("want repeat=5, got %d", inst.RepeatCount)
	}
}

func TestParseTimeInstruction_RepeatCount_Clamped(t *testing.T) {
	dur := 60.0
	inst := ParseTimeInstruction("first 5 seconds 99 times", dur)
	if inst.RepeatCount != maxTimelineRepeats {
		t.Errorf("want repeat clamped to %d, got %d", maxTimelineRepeats, inst.RepeatCount)
	}
	if len(inst.Warnings) == 0 {
		t.Error("expected clamp warning, got none")
	}
}

func TestParseTimeInstruction_Default(t *testing.T) {
	dur := 30.0
	inst := ParseTimeInstruction("make it look cinematic", dur)
	if inst.Start != 0 || inst.End != dur {
		t.Errorf("default should be whole video: want start=0 end=%.0f, got start=%.2f end=%.2f", dur, inst.Start, inst.End)
	}
	if inst.RepeatCount != 1 {
		t.Errorf("default repeat should be 1, got %d", inst.RepeatCount)
	}
}

func TestParseTimeInstruction_ClampRange_EndBeforeStart(t *testing.T) {
	dur := 60.0
	inst := ParseTimeInstruction("from 30 to 10 seconds", dur)
	// end <= start → fallback to whole video
	if inst.Start != 0 || inst.End != dur {
		t.Errorf("invalid range should fall back to whole video, got start=%.2f end=%.2f", inst.Start, inst.End)
	}
	if len(inst.Warnings) == 0 {
		t.Error("expected warning for invalid range, got none")
	}
}

func TestParseTimeInstruction_ZeroDuration(t *testing.T) {
	inst := ParseTimeInstruction("first 10 seconds", 0)
	// With zero duration everything clamps to 0; should not panic
	if inst.Start < 0 || inst.End < 0 {
		t.Errorf("unexpected negative values with zero duration: start=%.2f end=%.2f", inst.Start, inst.End)
	}
}

func TestParseSecondValue_MMSS(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"1:30", 90},
		{"0:30", 30},
		{"2:00", 120},
		{"1:30.5", 90.5},
	}
	for _, tc := range cases {
		got := parseSecondValue(tc.input)
		if got != tc.want {
			t.Errorf("parseSecondValue(%q): want %.1f, got %.1f", tc.input, tc.want, got)
		}
	}
}

func TestClampInt(t *testing.T) {
	if clampInt(5, 1, 10) != 5 {
		t.Error("5 should stay 5")
	}
	if clampInt(0, 1, 10) != 1 {
		t.Error("0 should clamp to 1")
	}
	if clampInt(15, 1, 10) != 10 {
		t.Error("15 should clamp to 10")
	}
}

func TestClampFloat(t *testing.T) {
	if clampFloat(5.0, 0, 10) != 5.0 {
		t.Error("5.0 should stay 5.0")
	}
	if clampFloat(-1.0, 0, 10) != 0 {
		t.Error("-1.0 should clamp to 0")
	}
	if clampFloat(15.0, 0, 10) != 10 {
		t.Error("15.0 should clamp to 10")
	}
}
