# `captions.srt`

## Purpose

`captions.srt` is a deterministic SRT subtitle file generated from `transcript.json`.

## Behavior

- uses transcript segments as cues
- preserves segment start/end timing
- numbers cues starting at `1`
- formats timestamps as `HH:MM:SS,mmm`
- uses segment text as caption text

If the transcript has no segments, the current generator writes an empty SRT file. This is safer than inventing captions.

## Example

```srt
1
00:00:00,000 --> 00:00:04,480
Okay, okay, okay.
```

## Limitations

Caption line wrapping, reading-speed optimization, speaker labels, and styling are not implemented yet.
