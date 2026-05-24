"""Core library: parse single-source agent markdown, transform, render.

Mirrors the aico Go implementation under cli/src/agent/ but in pure Python
with no third-party dependencies. Includes a small YAML reader/writer that
handles the subset of YAML used by agent frontmatter:

  - top-level mapping
  - scalars (string, int, float, bool, null)
  - block-style lists ("- item")
  - flow-style lists ("[a, b, c]")
  - nested mappings (block style, indented by 2 spaces)
  - inline flow mappings ("{key: value, ...}") — read-only; we never emit them

Anything beyond that is rejected with a clear error rather than silently
dropped, so authors know to simplify their frontmatter.
"""

from __future__ import annotations

import re
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

# ---------------------------------------------------------------------------
# Built-in mapping tables (kept in sync with cli/src/mapping/*.yaml)
# ---------------------------------------------------------------------------

# alias -> opencode provider/model-id. Reverse lookup is derived at runtime.
MODEL_ALIASES: dict[str, str] = {
    "sonnet": "anthropic/claude-sonnet-4-6",
    "opus": "anthropic/claude-opus-4-7",
    "haiku": "anthropic/claude-haiku-4-5-20251001",
}

# Claude named color -> hex (opencode uses hex).
COLOR_MAP: dict[str, str] = {
    "red": "#ef4444",
    "orange": "#f97316",
    "yellow": "#eab308",
    "green": "#22c55e",
    "blue": "#3b82f6",
    "purple": "#a855f7",
    "pink": "#ec4899",
    "gray": "#6b7280",
}


# ---------------------------------------------------------------------------
# Tiny YAML reader (subset)
# ---------------------------------------------------------------------------

class YAMLError(ValueError):
    pass


_TRUE = {"true", "yes", "on"}
_FALSE = {"false", "no", "off"}


def _parse_scalar(s: str) -> Any:
    s = s.strip()
    if s == "" or s == "~" or s.lower() == "null":
        return None
    if len(s) >= 2 and ((s[0] == s[-1] == '"') or (s[0] == s[-1] == "'")):
        return s[1:-1]
    low = s.lower()
    if low in _TRUE:
        return True
    if low in _FALSE:
        return False
    # int
    if re.fullmatch(r"-?\d+", s):
        try:
            return int(s)
        except ValueError:
            pass
    # float
    if re.fullmatch(r"-?\d+\.\d+([eE][+-]?\d+)?", s):
        try:
            return float(s)
        except ValueError:
            pass
    return s  # bare string


def _parse_flow_list(s: str) -> list[Any]:
    inner = s.strip()
    if not (inner.startswith("[") and inner.endswith("]")):
        raise YAMLError(f"expected flow list, got {s!r}")
    body = inner[1:-1].strip()
    if not body:
        return []
    # Split by commas not inside quotes — simple state machine.
    items: list[str] = []
    buf = []
    quote = None
    depth = 0
    for ch in body:
        if quote:
            buf.append(ch)
            if ch == quote:
                quote = None
            continue
        if ch in ('"', "'"):
            quote = ch
            buf.append(ch)
            continue
        if ch in "[{":
            depth += 1
            buf.append(ch)
            continue
        if ch in "]}":
            depth -= 1
            buf.append(ch)
            continue
        if ch == "," and depth == 0:
            items.append("".join(buf).strip())
            buf = []
            continue
        buf.append(ch)
    if buf:
        items.append("".join(buf).strip())
    return [_parse_scalar(x) for x in items]


def _parse_flow_map(s: str) -> dict[str, Any]:
    inner = s.strip()
    if not (inner.startswith("{") and inner.endswith("}")):
        raise YAMLError(f"expected flow map, got {s!r}")
    body = inner[1:-1].strip()
    if not body:
        return {}
    out: dict[str, Any] = {}
    # Split top-level commas like flow_list
    parts: list[str] = []
    buf = []
    quote = None
    depth = 0
    for ch in body:
        if quote:
            buf.append(ch)
            if ch == quote:
                quote = None
            continue
        if ch in ('"', "'"):
            quote = ch
            buf.append(ch)
            continue
        if ch in "[{":
            depth += 1
            buf.append(ch)
            continue
        if ch in "]}":
            depth -= 1
            buf.append(ch)
            continue
        if ch == "," and depth == 0:
            parts.append("".join(buf).strip())
            buf = []
            continue
        buf.append(ch)
    if buf:
        parts.append("".join(buf).strip())
    for p in parts:
        if ":" not in p:
            raise YAMLError(f"flow map entry missing ':': {p!r}")
        k, v = p.split(":", 1)
        out[k.strip()] = _parse_scalar(v)
    return out


def _strip_comment(line: str) -> str:
    """Strip a trailing `# ...` comment that is not inside quotes."""
    quote = None
    for i, ch in enumerate(line):
        if quote:
            if ch == quote:
                quote = None
            continue
        if ch in ('"', "'"):
            quote = ch
            continue
        if ch == "#" and (i == 0 or line[i - 1].isspace()):
            return line[:i].rstrip()
    return line.rstrip()


def yaml_loads(text: str) -> dict[str, Any]:
    """Parse the supported subset of YAML into a Python dict."""
    raw_lines = text.splitlines()
    lines: list[tuple[int, str]] = []
    for raw in raw_lines:
        line = _strip_comment(raw)
        if not line.strip():
            continue
        # leading-space count using tab-expansion for safety
        expanded = line.replace("\t", "    ")
        indent = len(expanded) - len(expanded.lstrip(" "))
        lines.append((indent, expanded.lstrip(" ")))

    pos = [0]

    def parse_block(indent: int) -> Any:
        # Decide list vs mapping by first sibling line.
        if pos[0] >= len(lines):
            return None
        first_indent, first_content = lines[pos[0]]
        if first_indent != indent:
            raise YAMLError(f"unexpected indent {first_indent}, want {indent}: {first_content!r}")
        if first_content.startswith("- "):
            return parse_list(indent)
        return parse_map(indent)

    def parse_list(indent: int) -> list[Any]:
        out: list[Any] = []
        while pos[0] < len(lines):
            cur_indent, content = lines[pos[0]]
            if cur_indent < indent:
                break
            if cur_indent != indent:
                raise YAMLError(f"unexpected indent in list: {content!r}")
            if not content.startswith("- "):
                break
            item = content[2:].strip()
            pos[0] += 1
            if item == "":
                # nested block
                if pos[0] < len(lines) and lines[pos[0]][0] > indent:
                    out.append(parse_block(lines[pos[0]][0]))
                else:
                    out.append(None)
            elif item.startswith("[") and item.endswith("]"):
                out.append(_parse_flow_list(item))
            elif item.startswith("{") and item.endswith("}"):
                out.append(_parse_flow_map(item))
            else:
                out.append(_parse_scalar(item))
        return out

    def parse_map(indent: int) -> dict[str, Any]:
        out: dict[str, Any] = {}
        while pos[0] < len(lines):
            cur_indent, content = lines[pos[0]]
            if cur_indent < indent:
                break
            if cur_indent != indent:
                raise YAMLError(f"unexpected indent in map: {content!r}")
            if content.startswith("- "):
                break
            if ":" not in content:
                raise YAMLError(f"expected 'key: value' line, got {content!r}")
            key, _, raw_val = content.partition(":")
            key = key.strip()
            val = raw_val.strip()
            pos[0] += 1
            if val == "":
                # nested block: read child lines if indented further
                if pos[0] < len(lines) and lines[pos[0]][0] > indent:
                    child_indent = lines[pos[0]][0]
                    out[key] = parse_block(child_indent)
                else:
                    out[key] = None
            elif val.startswith("[") and val.endswith("]"):
                out[key] = _parse_flow_list(val)
            elif val.startswith("{") and val.endswith("}"):
                out[key] = _parse_flow_map(val)
            else:
                out[key] = _parse_scalar(val)
        return out

    if not lines:
        return {}
    return parse_map(lines[0][0])


# ---------------------------------------------------------------------------
# Tiny YAML writer (block style, deterministic key order)
# ---------------------------------------------------------------------------

_NEEDS_QUOTING = re.compile(r"^[\s\-?:,\[\]\{\}#&*!|>'\"%@`]|[\s:#]$|: |^$")


def _emit_scalar(v: Any) -> str:
    if v is None:
        return "null"
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, (int, float)):
        return repr(v) if isinstance(v, float) else str(v)
    s = str(v)
    # Quote when YAML-significant chars are present or the string is empty.
    if s == "" or _NEEDS_QUOTING.search(s) or s.lower() in (_TRUE | _FALSE | {"null", "~"}):
        # double-quote with minimal escaping
        escaped = s.replace("\\", "\\\\").replace('"', '\\"')
        return f'"{escaped}"'
    if re.fullmatch(r"-?\d+(\.\d+)?", s):
        # numeric-looking strings need quoting
        return f'"{s}"'
    return s


def yaml_dumps(data: dict[str, Any], indent: int = 0) -> str:
    """Serialise the supported subset back to block-style YAML."""
    lines: list[str] = []
    pad = " " * indent
    for k, v in data.items():
        key = _emit_scalar(k) if not isinstance(k, str) or _NEEDS_QUOTING.search(k) else k
        if isinstance(v, dict):
            if not v:
                lines.append(f"{pad}{key}: {{}}")
            else:
                lines.append(f"{pad}{key}:")
                lines.append(yaml_dumps(v, indent + 2).rstrip("\n"))
        elif isinstance(v, list):
            if not v:
                lines.append(f"{pad}{key}: []")
            else:
                lines.append(f"{pad}{key}:")
                for item in v:
                    if isinstance(item, dict):
                        rendered = yaml_dumps(item, indent + 4).rstrip("\n").splitlines()
                        if rendered:
                            first = rendered[0].lstrip(" ")
                            lines.append(f"{pad}  - {first}")
                            for r in rendered[1:]:
                                lines.append(r)
                    elif isinstance(item, list):
                        # nested lists — emit inline for simplicity
                        lines.append(f"{pad}  - " + _emit_inline_list(item))
                    else:
                        lines.append(f"{pad}  - {_emit_scalar(item)}")
        else:
            lines.append(f"{pad}{key}: {_emit_scalar(v)}")
    return "\n".join(lines) + "\n"


def _emit_inline_list(items: list[Any]) -> str:
    return "[" + ", ".join(_emit_scalar(x) for x in items) + "]"


def model_to_opencode(alias: str) -> str | None:
    return MODEL_ALIASES.get(alias.lower())


def model_to_claude(provider_id: str) -> str | None:
    for alias, pid in MODEL_ALIASES.items():
        if pid == provider_id:
            return alias
    return None


def color_to_hex(name: str) -> str | None:
    return COLOR_MAP.get(name.lower())


def color_to_name(hex_value: str) -> str | None:
    lower = hex_value.lower()
    for name, hx in COLOR_MAP.items():
        if hx.lower() == lower:
            return name
    return None


# ---------------------------------------------------------------------------
# Source model + transform
# ---------------------------------------------------------------------------

USE_ONLY_BOTH = ""
USE_ONLY_CLAUDE = "claude"
USE_ONLY_OPENCODE = "opencode"


@dataclass
class Source:
    name: str = ""
    description: str = ""
    model: str = ""
    tools: list[str] = field(default_factory=list)
    color: str = ""
    useonly: str = ""
    claude: dict[str, Any] = field(default_factory=dict)
    opencode: dict[str, Any] = field(default_factory=dict)
    body: str = ""


def parse(content: str) -> Source:
    if content.startswith("﻿"):
        content = content[1:]

    stripped = content.lstrip()
    if not stripped.startswith("---"):
        raise ValueError("file must begin with --- frontmatter delimiter")

    rest = stripped[3:]
    idx = rest.find("\n---")
    if idx == -1:
        raise ValueError("missing closing --- frontmatter delimiter")

    raw_front = rest[:idx].strip()
    body = rest[idx + len("\n---"):].lstrip("\n").strip()

    data = yaml_loads(raw_front) or {}
    if not isinstance(data, dict):
        raise ValueError("frontmatter must be a YAML mapping")

    return Source(
        name=str(data.get("name", "") or ""),
        description=str(data.get("description", "") or ""),
        model=str(data.get("model", "") or ""),
        tools=list(data.get("tools") or []),
        color=str(data.get("color", "") or ""),
        useonly=str(data.get("useonly", "") or ""),
        claude=dict(data.get("claude") or {}),
        opencode=dict(data.get("opencode") or {}),
        body=body,
    )


def validate(src: Source) -> list[str]:
    errs: list[str] = []
    if not src.name.strip():
        errs.append("missing required field: name")
    if not src.description.strip():
        errs.append("missing required field: description")
    if src.useonly not in {"", "both", "all", USE_ONLY_CLAUDE, USE_ONLY_OPENCODE}:
        errs.append(
            f"invalid useonly value {src.useonly!r}: must be claude, opencode, both, all, or empty"
        )
    return errs


def normalize_useonly(u: str) -> str:
    return USE_ONLY_BOTH if u in ("", "both", "all") else u


def _resolve_target(src: Source, cli_only: str) -> str:
    return cli_only if cli_only else normalize_useonly(src.useonly)


def should_build_claude(src: Source, cli_only: str = "") -> bool:
    return _resolve_target(src, cli_only) in (USE_ONLY_BOTH, USE_ONLY_CLAUDE)


def should_build_opencode(src: Source, cli_only: str = "") -> bool:
    return _resolve_target(src, cli_only) in (USE_ONLY_BOTH, USE_ONLY_OPENCODE)


def _to_csv(v: Any) -> str:
    if isinstance(v, list):
        return ", ".join(str(x) for x in v)
    if isinstance(v, str):
        return v
    return str(v)


def _to_tool_map(v: Any, warnings: list[str]) -> dict[str, bool]:
    if isinstance(v, list):
        return {str(x).lower(): True for x in v}
    if isinstance(v, dict):
        out: dict[str, bool] = {}
        for k, b in v.items():
            if isinstance(b, bool):
                out[str(k).lower()] = b
        return out
    warnings.append(f"opencode tools has unexpected type {type(v).__name__}, dropping")
    return {}


_CLAUDE_FIELDS = [
    "name", "description", "model", "tools", "color",
    "permissionMode", "disallowedTools", "maxTurns", "skills",
    "mcpServers", "hooks", "memory", "background", "effort",
    "isolation", "initialPrompt",
]

_OPENCODE_FIELDS = [
    "description", "mode", "model", "tools", "color",
    "temperature", "top_p", "steps", "permission", "disable", "hidden",
]


def transform_claude(src: Source, strict: bool = False) -> tuple[dict[str, Any], list[str]]:
    warnings: list[str] = []
    out: dict[str, Any] = {"name": src.name, "description": src.description}

    if src.model:
        if "/" in src.model:
            alias = model_to_claude(src.model)
            if alias:
                out["model"] = alias
            else:
                msg = f"model {src.model!r} has no Claude alias mapping, using as-is"
                if strict:
                    raise ValueError(msg)
                warnings.append(msg)
                out["model"] = src.model
        else:
            out["model"] = src.model

    if src.tools:
        out["tools"] = ", ".join(src.tools)

    if src.color:
        if src.color.startswith("#"):
            name = color_to_name(src.color)
            if name:
                out["color"] = name
            else:
                msg = f"color hex {src.color!r} has no Claude name mapping, dropping"
                if strict:
                    raise ValueError(msg)
                warnings.append(msg)
        else:
            out["color"] = src.color

    for k, v in (src.claude or {}).items():
        if k == "permissionMode":
            out["permissionMode"] = str(v)
        elif k == "disallowedTools":
            out["disallowedTools"] = _to_csv(v)
        elif k == "maxTurns":
            out["maxTurns"] = v
        elif k == "skills":
            out["skills"] = v
        elif k == "mcpServers":
            out["mcpServers"] = v
        elif k == "hooks":
            out["hooks"] = v
        elif k == "memory":
            out["memory"] = str(v)
        elif k == "background":
            out["background"] = v
        elif k == "effort":
            out["effort"] = str(v)
        elif k == "isolation":
            out["isolation"] = str(v)
        elif k == "initialPrompt":
            out["initialPrompt"] = str(v)
        elif k == "model":
            out["model"] = str(v)
        elif k == "tools":
            out["tools"] = _to_csv(v)
        elif k == "color":
            out["color"] = str(v)
        else:
            warnings.append(f"unknown claude override field {k!r}, ignoring")

    return _ordered(out, _CLAUDE_FIELDS), warnings


def transform_opencode(src: Source, strict: bool = False) -> tuple[dict[str, Any], list[str]]:
    warnings: list[str] = []
    out: dict[str, Any] = {"description": src.description}

    if src.model:
        if "/" in src.model:
            out["model"] = src.model
        else:
            pid = model_to_opencode(src.model)
            if pid:
                out["model"] = pid
            else:
                msg = f"model alias {src.model!r} has no opencode provider/model-id mapping, using as-is"
                if strict:
                    raise ValueError(msg)
                warnings.append(msg)
                out["model"] = src.model

    if src.tools:
        out["tools"] = {t.lower(): True for t in src.tools}

    if src.color:
        if src.color.startswith("#"):
            out["color"] = src.color
        else:
            hx = color_to_hex(src.color)
            if hx:
                out["color"] = hx
            else:
                msg = f"color name {src.color!r} has no hex mapping, dropping"
                if strict:
                    raise ValueError(msg)
                warnings.append(msg)

    for k, v in (src.opencode or {}).items():
        if k == "mode":
            out["mode"] = str(v)
        elif k == "temperature":
            out["temperature"] = v
        elif k == "top_p":
            out["top_p"] = v
        elif k == "steps":
            out["steps"] = v
        elif k == "permission":
            if isinstance(v, dict):
                out["permission"] = v
            else:
                warnings.append(f"opencode.permission has unexpected type {type(v).__name__}")
        elif k == "disable":
            out["disable"] = v
        elif k == "hidden":
            out["hidden"] = v
        elif k == "model":
            out["model"] = str(v)
        elif k == "tools":
            out["tools"] = _to_tool_map(v, warnings)
        elif k == "color":
            out["color"] = str(v)
        else:
            warnings.append(f"unknown opencode override field {k!r}, ignoring")

    return _ordered(out, _OPENCODE_FIELDS), warnings


def _ordered(d: dict[str, Any], order: list[str]) -> dict[str, Any]:
    out: dict[str, Any] = {}
    for k in order:
        if k in d:
            out[k] = d[k]
    for k, v in d.items():
        if k not in out:
            out[k] = v
    return out


def render(front: dict[str, Any], body: str) -> str:
    parts = ["---\n", yaml_dumps(front), "---\n"]
    if body:
        parts.append("\n")
        parts.append(body)
        parts.append("\n")
    return "".join(parts)


def write_file(path: Path, data: str, src_mtime: float, force: bool) -> bool:
    path.parent.mkdir(parents=True, exist_ok=True)
    if not force and path.exists():
        if path.stat().st_mtime >= src_mtime:
            return False
    path.write_text(data, encoding="utf-8")
    return True


def load_source(path: Path) -> Source:
    src = parse(path.read_text(encoding="utf-8"))
    if not src.name:
        src.name = path.stem
    return src


def eprint(*args: Any) -> None:
    print(*args, file=sys.stderr)


EXIT_OK = 0
EXIT_VALIDATION = 1
EXIT_IO = 2
EXIT_MAPPING = 3
