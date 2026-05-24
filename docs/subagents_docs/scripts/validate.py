#!/usr/bin/env python3
"""Validate one or more agent source markdown files. No file writes.

Usage:
    python3 validate.py <source.md> [<source.md> ...]
"""

from __future__ import annotations

import sys
from pathlib import Path

from agent_lib import EXIT_IO, EXIT_OK, EXIT_VALIDATION, eprint, load_source, validate


def main(argv: list[str]) -> int:
    if not argv:
        eprint("usage: validate.py <source.md> [<source.md> ...]")
        return EXIT_IO

    status = EXIT_OK
    for arg in argv:
        path = Path(arg)
        if not path.is_file():
            eprint(f"error: not a file: {path}")
            status = EXIT_IO
            continue
        try:
            src = load_source(path)
        except Exception as e:
            eprint(f"error [{path}]: parse failed: {e}")
            status = EXIT_VALIDATION
            continue

        errs = validate(src)
        if errs:
            for e in errs:
                eprint(f"validation [{path}]: {e}")
            status = EXIT_VALIDATION
        else:
            print(f"ok {path}")

    return status


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
