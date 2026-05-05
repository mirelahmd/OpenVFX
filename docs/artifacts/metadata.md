# `metadata.json`

## Purpose

`metadata.json` is the raw `ffprobe` JSON output for the input media file.

The current workflow does not normalize this artifact. Downstream tools may derive summaries from it, but the raw artifact is preserved for replay and debugging.

## Generation

The Go control plane currently runs:

```sh
ffprobe -v quiet -print_format json -show_format -show_streams <input-file>
```

The command output is written directly to:

```text
.byom-video/runs/<run_id>/metadata.json
```

## Contract

- Preserve raw `ffprobe` JSON.
- Do not rewrite it into an internal normalized schema yet.
- Downstream readers should tolerate fields varying by media type, codec, and ffprobe version.
- Summaries such as duration and stream counts may be derived from this artifact.

## Example

```json
{
  "streams": [
    {
      "index": 0,
      "codec_name": "h264",
      "codec_type": "video",
      "width": 320,
      "height": 180,
      "duration": "2.000000"
    },
    {
      "index": 1,
      "codec_name": "aac",
      "codec_type": "audio",
      "sample_rate": "44100",
      "channels": 1,
      "duration": "2.000000"
    }
  ],
  "format": {
    "filename": "examples/fixtures/tiny.mp4",
    "nb_streams": 2,
    "format_name": "mov,mp4,m4a,3gp,3g2,mj2",
    "duration": "2.000000",
    "size": "29679"
  }
}
```

The actual `metadata.json` may include many more ffprobe fields.
