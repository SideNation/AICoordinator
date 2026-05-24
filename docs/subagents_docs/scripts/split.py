#!/usr/bin/env python3
"""Split a single-source agent markdown into Claude + opencode files.

Usage:
    python3 split.py <source.md> [--out-dir DIR] [--only claude|opencode]
                                 [--dry-run] [--force] [--strict]
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from agent_lib import (
    EXIT_IO, EXIT_MAPPING, EXIT_OK, EXIT_VALIDATION,
    eprint, load_source, render, should_build_claude, should_build_opencode,
    transform_claude, transform_opencode, validate, write_file,
)


def main(argv: list[str]) -> int:
    p = argparse.ArgumentParser(description="Split agent source markdown into Claude + opencode files.")
    p.add_argument("source", help="Path to agents/<name>.md")
    p.add_argument("--out-dir", default="packages/agents", help="Output root (default: packages/agents)")
    p.add_argument("--only", choices=["claude", "opencode"], default="", help="Limit output to one target")
    p.add_argument("--dry-run", action="store_true", help="Print to stdout instead of writing")
    p.add_argument("--force", action="store_true", help="Overwrite even if destination is newer")
    p.add_argument("--strict", action="store_true", help="Fail on unmappable values")
    args = p.parse_args(argv)

    src_path = Path(args.source)
    if not src_path.is_file():
        eprint(f"error: source file not found: {src_path}")
        return EXIT_IO

    try:
        src = load_source(src_path)
    except Exception as e:
        eprint(f"error: parse failed: {e}")
        return EXIT_VALIDATION

    errs = validate(src)
    if errs:
        for e in errs:
            eprint(f"validation: {e}")
        return EXIT_VALIDATION

    out_root = Path(args.out_dir)
    src_mtime = src_path.stat().st_mtime
    wrote_any = False

    targets = []
    if should_build_claude(src, args.only):
        targets.append("claude")
    if should_build_opencode(src, args.only):
        targets.append("opencode")

    for target in targets:
        try:
            if target == "claude":
                front, warns = transform_claude(src, strict=args.strict)
            else:
                front, warns = transform_opencode(src, strict=args.strict)
        except ValueError as e:
            eprint(f"mapping: {e}")
            return EXIT_MAPPING

        for w in warns:
            eprint(f"warning ({target}): {w}")

        rendered = render(front, src.body)
        dest = out_root / target / f"{src.name}.md"

        if args.dry_run:
            print(f"--- {dest} ---")
            print(rendered)
            continue

        try:
            wrote = write_file(dest, rendered, src_mtime, force=args.force)
        except OSError as e:
            eprint(f"io: {e}")
            return EXIT_IO

        if wrote:
            wrote_any = True
            print(f"wrote {dest}")
        else:
            print(f"up-to-date {dest}")

    if not args.dry_run and not wrote_any and targets:
        # Nothing written but everything up-to-date is still success.
        pass

    return EXIT_OK


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
