#!/usr/bin/env python3
"""Build all agent sources in a directory into Claude + opencode outputs.

Usage:
    python3 build.py [--src DIR] [--out-dir DIR] [--only claude|opencode]
                     [--dry-run] [--force] [--strict] [--prune]
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
    p = argparse.ArgumentParser(description="Build all agent sources to Claude + opencode outputs.")
    p.add_argument("--src", default="agents", help="Source directory of single-source markdowns")
    p.add_argument("--out-dir", default="packages/agents", help="Output root")
    p.add_argument("--only", choices=["claude", "opencode"], default="")
    p.add_argument("--dry-run", action="store_true")
    p.add_argument("--force", action="store_true")
    p.add_argument("--strict", action="store_true")
    p.add_argument("--prune", action="store_true",
                   help="Remove output files for targets excluded by useonly")
    args = p.parse_args(argv)

    src_dir = Path(args.src)
    if not src_dir.is_dir():
        eprint(f"error: source directory not found: {src_dir}")
        return EXIT_IO

    out_root = Path(args.out_dir)
    sources = sorted(src_dir.glob("*.md"))
    if not sources:
        eprint(f"warning: no .md files found in {src_dir}")
        return EXIT_OK

    overall_status = EXIT_OK

    for src_path in sources:
        try:
            src = load_source(src_path)
        except Exception as e:
            eprint(f"error [{src_path}]: parse failed: {e}")
            overall_status = EXIT_VALIDATION
            continue

        errs = validate(src)
        if errs:
            for e in errs:
                eprint(f"validation [{src_path}]: {e}")
            overall_status = EXIT_VALIDATION
            continue

        src_mtime = src_path.stat().st_mtime
        targets_built = []
        targets_skipped = []

        for target in ("claude", "opencode"):
            wants = (target == "claude" and should_build_claude(src, args.only)) or \
                    (target == "opencode" and should_build_opencode(src, args.only))
            if not wants:
                targets_skipped.append(target)
                continue

            try:
                if target == "claude":
                    front, warns = transform_claude(src, strict=args.strict)
                else:
                    front, warns = transform_opencode(src, strict=args.strict)
            except ValueError as e:
                eprint(f"mapping [{src_path} -> {target}]: {e}")
                overall_status = EXIT_MAPPING
                continue

            for w in warns:
                eprint(f"warning [{src.name} -> {target}]: {w}")

            rendered = render(front, src.body)
            dest = out_root / target / f"{src.name}.md"

            if args.dry_run:
                print(f"--- {dest} ---")
                print(rendered)
                continue

            try:
                wrote = write_file(dest, rendered, src_mtime, force=args.force)
            except OSError as e:
                eprint(f"io [{dest}]: {e}")
                overall_status = EXIT_IO
                continue

            targets_built.append((target, dest, wrote))

        if args.prune and not args.dry_run:
            for target in targets_skipped:
                stale = out_root / target / f"{src.name}.md"
                if stale.exists():
                    try:
                        stale.unlink()
                        print(f"pruned {stale}")
                    except OSError as e:
                        eprint(f"io [{stale}]: {e}")
                        overall_status = EXIT_IO

        if not args.dry_run:
            for target, dest, wrote in targets_built:
                print(f"{'wrote' if wrote else 'up-to-date'} {dest}")

    return overall_status


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
