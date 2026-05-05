from __future__ import annotations

import json
from pathlib import Path
from typing import Any


def transcribe(input_path: str, run_dir: str, model_size: str = "tiny") -> Path:
    source = Path(input_path)
    target_dir = Path(run_dir)

    if not source.is_file():
        raise FileNotFoundError(f"input file does not exist: {source}")
    if not target_dir.is_dir():
        raise FileNotFoundError(f"run directory does not exist: {target_dir}")

    try:
        from faster_whisper import WhisperModel
    except ImportError as exc:
        raise RuntimeError(
            "faster-whisper is not installed. Install transcription dependencies with "
            "`python3 -m pip install -e \"workers[transcribe]\"`."
        ) from exc

    model = WhisperModel(model_size)
    segments_iter, info = model.transcribe(str(source))
    segments = list(segments_iter)

    transcript = {
        "schema_version": "transcript.v1",
        "source": {
            "input_path": str(source),
            "mode": "real",
            "engine": "faster-whisper",
            "model_size": model_size,
        },
        "language": _get_attr(info, "language", "unknown") or "unknown",
        "duration_seconds": _get_attr(info, "duration", None),
        "segments": [
            {
                "id": f"seg_{index + 1:04d}",
                "start": float(segment.start),
                "end": float(segment.end),
                "text": segment.text.strip(),
            }
            for index, segment in enumerate(segments)
        ],
    }

    output_path = target_dir / "transcript.json"
    output_path.write_text(json.dumps(transcript, indent=2) + "\n", encoding="utf-8")
    return output_path


def _get_attr(value: Any, name: str, default: Any) -> Any:
    return getattr(value, name, default)
