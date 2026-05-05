from __future__ import annotations

import argparse
import sys

from .transcribe_stub import transcribe_stub


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="byom-video-worker")
    subparsers = parser.add_subparsers(dest="command", required=True)

    transcribe_stub_parser = subparsers.add_parser("transcribe-stub")
    transcribe_stub_parser.add_argument("--input", required=True)
    transcribe_stub_parser.add_argument("--run-dir", required=True)

    transcribe_parser = subparsers.add_parser("transcribe")
    transcribe_parser.add_argument("--input", required=True)
    transcribe_parser.add_argument("--run-dir", required=True)
    transcribe_parser.add_argument("--model-size", default="tiny")

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    try:
        if args.command == "transcribe-stub":
            output_path = transcribe_stub(args.input, args.run_dir)
            print(f"transcribe-stub completed: wrote {output_path}")
            return 0
        if args.command == "transcribe":
            from .transcribe import transcribe

            output_path = transcribe(args.input, args.run_dir, args.model_size)
            print(f"transcribe completed: wrote {output_path}")
            return 0
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    parser.error(f"unknown command {args.command}")
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
