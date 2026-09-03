#!/usr/bin/env python3
"""Structural checker for bilingual documentation under docs/.

For every *.md under the root directory (default: the repository's docs/):

  - each document must have a paired translation <basename>.zh.md
    (and orphan translations without a source are failures);
  - heading counts at each level must match between the pair;
  - fenced code block counts must match between the pair.

Parsing is deliberately structural. Fenced regions are cut out first and
headings are counted only outside them: a '#' comment line inside a code
fence is not a heading. Known limitations, accepted by design: four-backtick
nested fences and 4-space indented code blocks are not specially handled
(this project's documents use neither).

Structural checks cannot catch semantic translation drift; that remains the
responsibility of change review (see docs/process/development-process.md).

Usage:
    python3 openspec/tools/docs_sync.py [--root <dir>]

Exit codes: 0 = all pairs present and aligned, 1 = failures found,
2 = usage error.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

FENCE_RE = re.compile(r"^```")
HEADING_RE = re.compile(r"^(#{1,6})\s")
ZH_SUFFIX = ".zh.md"


def parse_root(argv: list[str]) -> Path:
    """Resolve the docs root from --root, defaulting to the repo's docs/."""
    default = Path(__file__).resolve().parent.parent.parent / "docs"
    if "--root" in argv:
        index = argv.index("--root")
        if index + 1 >= len(argv):
            print("FAIL: --root requires a directory argument")
            sys.exit(2)
        return Path(argv[index + 1]).resolve()
    return default


def profile(text: str) -> tuple[dict[int, int], int]:
    """Return (heading count per level, fence line count) outside fences."""
    headings: dict[int, int] = {}
    fences = 0
    in_fence = False
    for line in text.splitlines():
        if FENCE_RE.match(line):
            fences += 1
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        match = HEADING_RE.match(line)
        if match:
            level = len(match.group(1))
            headings[level] = headings.get(level, 0) + 1
    return headings, fences


def describe(headings: dict[int, int]) -> str:
    """Render heading counts as 'H1=2 H2=5' for failure messages."""
    if not headings:
        return "no headings"
    return " ".join(f"H{level}={count}" for level, count in sorted(headings.items()))


def main() -> int:
    root = parse_root(sys.argv)
    if not root.is_dir():
        print(f"FAIL: docs root not found: {root}")
        return 1

    markdown_files = sorted(root.rglob("*.md"))
    sources = [path for path in markdown_files if not path.name.endswith(ZH_SUFFIX)]
    translations = [path for path in markdown_files if path.name.endswith(ZH_SUFFIX)]

    if not sources and not translations:
        print(f"OK: no markdown documents found under {root}")
        return 0

    failures = 0
    paired = 0

    for source in sources:
        translation = source.with_name(source.stem + ZH_SUFFIX)
        rel = source.relative_to(root)
        if not translation.is_file():
            print(f"FAIL: {rel}: missing paired translation {translation.name}")
            failures += 1
            continue
        paired += 1
        src_headings, src_fences = profile(source.read_text(encoding="utf-8"))
        zh_headings, zh_fences = profile(translation.read_text(encoding="utf-8"))
        if src_headings != zh_headings:
            print(f"FAIL: {rel}: heading mismatch — en [{describe(src_headings)}] vs zh [{describe(zh_headings)}]")
            failures += 1
        if src_fences != zh_fences:
            print(f"FAIL: {rel}: fenced code block mismatch — en {src_fences} vs zh {zh_fences}")
            failures += 1

    for translation in translations:
        source = translation.with_name(translation.name[: -len(ZH_SUFFIX)] + ".md")
        if not source.is_file():
            print(f"FAIL: {translation.relative_to(root)}: orphan translation (no source {source.name})")
            failures += 1

    if failures:
        print(f"\n{failures} failure(s) across {paired} paired document(s); root={root}")
        return 1

    print(f"OK: {paired} document pair(s) aligned; root={root}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
