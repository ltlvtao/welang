#!/usr/bin/env python3
"""Structural validator for welang openspec change directories.

Checks artifact completeness per change status, required proposal headers,
spec-delta scenario structure, task verification format, and consistency
between docs/spec chapters and the diagnostic code registry
(docs/spec/diagnostics.toml). Structural validation only; semantic review
is carried by the welang-change-review and welang-code-review skills and
is NOT replaced by this script.

Usage:
    python3 openspec/tools/validate.py --all [--strict]
    python3 openspec/tools/validate.py <change-name> [--strict]

Exit codes: 0 = pass, 1 = validation failures found.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

VALID_STATUSES = {"candidate", "ready", "active", "complete", "archived"}
VALID_LAYERS = {"spec", "compiler", "stdlib", "tooling", "benchmark", "process", "docs"}

# candidate only needs proposal; these statuses need the full four-artifact set.
FULL_ARTIFACT_STATUSES = {"ready", "active", "complete"}

PROPOSAL_HEADERS = [
    "## Why",
    "## 目标与非目标",
    "## What Changes",
    "## 影响层",
    "## 影响范围",
]

DELTA_HEADERS = [
    "## ADDED Requirements",
    "## MODIFIED Requirements",
    "## REMOVED Requirements",
    "## RENAMED Requirements",
]


class Fail:
    def __init__(self, path: Path, reason: str) -> None:
        self.path = path
        self.reason = reason

    def __str__(self) -> str:
        return f"{self.path}: {self.reason}"


def parse_change_yaml(text: str) -> dict[str, str]:
    """Minimal flat YAML reader: 'key: value' lines only."""
    result: dict[str, str] = {}
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        if ":" not in line:
            continue
        key, _, value = line.partition(":")
        result[key.strip()] = value.strip()
    return result


def validate_change(change_dir: Path, strict: bool) -> list[Fail]:
    fails: list[Fail] = []
    rel = lambda p: p  # keep paths absolute-ish for clear reporting

    # --- change.yaml -------------------------------------------------
    yaml_path = change_dir / "change.yaml"
    if not yaml_path.is_file():
        return [Fail(rel(change_dir), "missing change.yaml")]
    fields = parse_change_yaml(yaml_path.read_text(encoding="utf-8"))

    name = fields.get("name", "")
    if name != change_dir.name:
        fails.append(Fail(rel(yaml_path), f"name '{name}' does not match directory '{change_dir.name}'"))

    status = fields.get("status", "")
    if status not in VALID_STATUSES:
        fails.append(Fail(rel(yaml_path), f"invalid status '{status}' (expected one of {sorted(VALID_STATUSES)})"))

    layers_raw = fields.get("layers", "")
    layers = {part.strip() for part in layers_raw.strip("[]").split(",") if part.strip()}
    if not layers or not layers <= VALID_LAYERS:
        fails.append(Fail(rel(yaml_path), f"invalid layers {sorted(layers)} (expected subset of {sorted(VALID_LAYERS)})"))

    # Language-behavior changes must carry a spec delta; enforce the cheap
    # structural half here (behavioral layers listed without 'spec').
    behavior_layers = layers & {"compiler", "stdlib", "tooling"}
    if strict and behavior_layers and "spec" not in layers:
        # Allowed only when proposal explicitly justifies an internals-only
        # change; the justification itself is checked by semantic review.
        proposal = change_dir / "proposal.md"
        text = proposal.read_text(encoding="utf-8") if proposal.is_file() else ""
        if "非目标" not in text and "不改变语言行为" not in text:
            fails.append(Fail(rel(yaml_path), "layers touch compiler/stdlib/tooling without 'spec'; justify internals-only scope in proposal"))

    # --- proposal.md ---------------------------------------------------
    proposal = change_dir / "proposal.md"
    if not proposal.is_file():
        fails.append(Fail(rel(change_dir), "missing proposal.md"))
    else:
        text = proposal.read_text(encoding="utf-8")
        for header in PROPOSAL_HEADERS:
            if header not in text:
                fails.append(Fail(rel(proposal), f"missing required header '{header}'"))

    # --- full artifacts for ready/active/complete -----------------------
    if status in FULL_ARTIFACT_STATUSES:
        for required in ("design.md", "tasks.md"):
            if not (change_dir / required).is_file():
                fails.append(Fail(rel(change_dir), f"status '{status}' requires {required}"))
        spec_files = sorted((change_dir / "specs").glob("*/spec.md")) if (change_dir / "specs").is_dir() else []
        if not spec_files:
            fails.append(Fail(rel(change_dir), f"status '{status}' requires at least one specs/<capability>/spec.md"))

    # --- spec deltas ----------------------------------------------------
    for spec_file in sorted(change_dir.glob("specs/*/spec.md")):
        text = spec_file.read_text(encoding="utf-8")
        if not any(header in text for header in DELTA_HEADERS):
            fails.append(Fail(rel(spec_file), "no ADDED/MODIFIED/REMOVED/RENAMED Requirements section"))
        # Every Requirement must have at least one Scenario with WHEN/THEN.
        req_blocks = re.split(r"^### Requirement:", text, flags=re.MULTILINE)[1:]
        for index, block in enumerate(req_blocks, start=1):
            title = block.splitlines()[0].strip() if block.splitlines() else f"#{index}"
            scenarios = re.findall(r"^#### Scenario:(.*?)(?=^#### |^### |\Z)", block, flags=re.MULTILINE | re.DOTALL)
            if not scenarios:
                fails.append(Fail(rel(spec_file), f"Requirement '{title}' has no Scenario"))
            for scenario in scenarios:
                if "**WHEN**" not in scenario or "**THEN**" not in scenario:
                    fails.append(Fail(rel(spec_file), f"Scenario under Requirement '{title}' lacks WHEN/THEN"))

    # --- tasks.md --------------------------------------------------------
    tasks = change_dir / "tasks.md"
    if tasks.is_file():
        lines = tasks.read_text(encoding="utf-8").splitlines()
        checkbox = re.compile(r"^\s*- \[( |x)\]\s*(.*)$")
        for i, line in enumerate(lines):
            match = checkbox.match(line)
            if not match:
                continue
            checked, _title = match.groups()
            # Look ahead until the next checkbox for the source/verify pair.
            lookahead: list[str] = []
            for follow in lines[i + 1:]:
                if checkbox.match(follow):
                    break
                lookahead.append(follow)
            block = "\n".join(lookahead)
            if "来源：" not in block and "来源:" not in block:
                fails.append(Fail(rel(tasks), f"line {i + 1}: checkbox lacks '来源：' line"))
            if "验证：" not in block and "验证:" not in block:
                fails.append(Fail(rel(tasks), f"line {i + 1}: checkbox lacks '验证：' line"))
            elif strict and checked == "x":
                # Strict mode: a checked task must carry non-empty verification.
                # Strip the 验证 prefix (fullwidth or ASCII colon variant) from
                # each matching line and concatenate the remainder; do not
                # partition on later colons inside the verification text.
                parts: list[str] = []
                for follow in lookahead:
                    stripped = follow.strip()
                    for prefix in ("验证：", "验证:"):
                        if stripped.startswith(prefix):
                            parts.append(stripped[len(prefix):])
                            break
                content = "".join(parts).strip()
                if not content:
                    fails.append(Fail(rel(tasks), f"line {i + 1}: checked task has empty verification"))

    return fails


def validate_registry(docs_root: Path) -> list[Fail]:
    """Consistency checks between docs/spec chapters and diagnostics.toml.

    The registry is the entry authority for every allocated diagnostic code
    and the segment authority for code ranges (spec chapter 99). Checks:
    (a) TOML parses and every entry carries the seven required fields with
    a key/severity prefix match; (b) every diagnostic usage in spec chapter
    markdown (a code token immediately followed by ':') has a registry
    entry — range endpoints like E0001-E0099 carry no colon and are not
    usages; (c) every entry's owner file and Requirement title resolve;
    (d) every entry's number lies in a segment owned by its owner chapter;
    (e) E and W share one number space, so no number is allocated twice.
    """
    fails: list[Fail] = []
    spec_dir = docs_root / "spec"
    registry_path = spec_dir / "diagnostics.toml"
    if not registry_path.is_file() or not spec_dir.is_dir():
        return fails
    try:
        import tomllib
    except ModuleNotFoundError:
        return [Fail(registry_path, "registry checks require Python 3.11+ (tomllib)")]
    try:
        registry = tomllib.loads(registry_path.read_text(encoding="utf-8"))
    except tomllib.TOMLDecodeError as exc:
        return [Fail(registry_path, f"invalid TOML: {exc}")]

    segments = registry.get("segments", {})
    entries = registry.get("diagnostic", {})
    required_fields = ("severity", "title", "description", "remediation", "owner", "requirement", "allocated")

    parsed_segments: list[tuple[int, int, str]] = []
    for key, data in segments.items():
        match = re.fullmatch(r"([EW])(\d{4})-([EW])(\d{4})", key)
        if not match:
            fails.append(Fail(registry_path, f"segment key '{key}' is not a CODE-CODE range"))
            continue
        parsed_segments.append((int(match.group(2)), int(match.group(4)), str(data.get("owner", ""))))
        for field in ("domain", "owner"):
            if field not in data:
                fails.append(Fail(registry_path, f"segment '{key}' lacks '{field}'"))

    seen_numbers: dict[int, str] = {}
    for code, entry in entries.items():
        if not re.fullmatch(r"[EW]\d{4}", code):
            fails.append(Fail(registry_path, f"entry key '{code}' is not an E/W + 4-digit code"))
            continue
        for field in required_fields:
            if field not in entry or not str(entry[field]).strip():
                fails.append(Fail(registry_path, f"entry '{code}' lacks non-empty '{field}'"))
        severity = str(entry.get("severity", ""))
        if severity not in ("error", "warning"):
            fails.append(Fail(registry_path, f"entry '{code}' severity must be 'error' or 'warning'"))
        elif severity != ("error" if code[0] == "E" else "warning"):
            fails.append(Fail(registry_path, f"entry '{code}' prefix does not match severity '{severity}'"))
        number = int(code[1:])
        if number in seen_numbers:
            fails.append(Fail(registry_path, f"entries '{seen_numbers[number]}' and '{code}' share number {number:04d}: E and W share one number space"))
        else:
            seen_numbers[number] = code
        owner = str(entry.get("owner", ""))
        owner_file = spec_dir / f"{owner}.md"
        if not owner_file.is_file():
            fails.append(Fail(registry_path, f"entry '{code}' owner file docs/spec/{owner}.md does not exist"))
            continue
        owner_text = owner_file.read_text(encoding="utf-8")
        requirement = str(entry.get("requirement", ""))
        if requirement and f"### Requirement: {requirement}" not in owner_text:
            fails.append(Fail(registry_path, f"entry '{code}' requirement '{requirement}' not found in docs/spec/{owner}.md"))
        if not any(lo <= number <= hi and seg_owner == owner for lo, hi, seg_owner in parsed_segments):
            fails.append(Fail(registry_path, f"entry '{code}' does not lie in a segment owned by '{owner}'"))

    # A diagnostic usage is a code token immediately followed by ':' — the
    # invocation form used in Scenario text. The lookbehind excludes the
    # upper endpoint of dash-written ranges (E0100-E0199: ...).
    for md in sorted(spec_dir.rglob("*.md")):
        text = md.read_text(encoding="utf-8")
        for code in sorted(set(re.findall(r"(?<![-–])\b([EW]\d{4}):", text))):
            if code not in entries:
                fails.append(Fail(md, f"diagnostic usage '{code}:' has no registry entry in docs/spec/diagnostics.toml"))
    return fails


def main() -> int:
    args = sys.argv[1:]
    strict = "--strict" in args
    args = [a for a in args if not a.startswith("--")]

    root = Path(__file__).resolve().parent.parent  # .../openspec
    changes_root = root / "changes"

    if args and args[0] == "--all":
        args = []

    if args:
        targets = [changes_root / name for name in args]
        for target in targets:
            if not target.is_dir():
                print(f"FAIL: change directory not found: {target}")
                return 1
    else:
        targets = sorted(p for p in changes_root.iterdir() if p.is_dir() and p.name != "archive") if changes_root.is_dir() else []

    registry_note = ""
    all_fails: list[Fail] = []
    docs_root = Path(__file__).resolve().parents[2] / "docs"
    if docs_root.is_dir():
        reg_fails = validate_registry(docs_root)
        all_fails.extend(reg_fails)
        registry_note = "; registry clean" if not reg_fails else f"; {len(reg_fails)} registry failure(s)"

    if not targets:
        print(f"OK: no changes found (nothing to validate){registry_note}")
        return 1 if all_fails else 0

    for target in targets:
        all_fails.extend(validate_change(target, strict))

    if all_fails:
        for fail in all_fails:
            print(f"FAIL: {fail}")
        print(f"\n{len(all_fails)} failure(s) across {len(targets)} change(s){registry_note}; mode={'strict' if strict else 'default'}")
        return 1

    print(f"OK: {len(targets)} change(s) valid{registry_note}; mode={'strict' if strict else 'default'}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
