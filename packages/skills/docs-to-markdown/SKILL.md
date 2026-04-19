---
name: docs-to-markdown
description: Convert a public documentation site (framework/library official docs, etc.) into a tree of local Markdown files. Triggers when the user asks to "수집/변환/미러/크롤/markdownify" a docs URL or explicitly invokes /docs-to-markdown. Drives two subagents (link-harvester, markdown-converter) via Playwright MCP and updates lock.yaml.
---

# docs-to-markdown

Use this skill when the user wants to mirror a documentation site to local
Markdown. You own argument parsing, directory layout, subagent orchestration,
and the final `lock.yaml` update. The actual browser work lives in the two
subagents — do not open a browser from this skill directly.

## When to invoke

- User message contains a docs URL and words like "수집", "변환", "미러",
  "크롤", "markdown으로", "docs-to-markdown".
- User explicitly runs `/docs-to-markdown <url> --name <name>`.

## Arguments

```
/docs-to-markdown <url> --name <name> [flags]
```

| Arg                    | Type   | Required | Default      | Notes                                        |
| ---------------------- | ------ | -------- | ------------ | -------------------------------------------- |
| `url`                  | string | yes      | —            | Entry URL of the docs (a nav must be visible)|
| `--name`               | string | yes      | —            | Output folder `packages/docs/<name>/`        |
| `--nav-selector`       | string | no       | auto         | Override nav root CSS selector               |
| `--content-selector`   | string | no       | auto         | Override content container CSS selector      |
| `--include`            | glob   | no       | `*`          | URL path include filter                      |
| `--exclude`            | glob   | no       | —            | URL path exclude filter                      |
| `--resume`             | bool   | no       | `true`       | Merge with existing `links.json`             |
| `--rate-limit-ms`      | int    | no       | `300`        | Delay between page fetches                   |
| `--max-errors`         | int    | no       | `20`         | Abort threshold during conversion            |
| `--video-mode`         | enum   | no       | `link`       | v1 supports `link` only                      |

If `url` or `--name` is missing, prompt the user once with a brief example:
`/docs-to-markdown https://react.dev/reference --name react`.

## Procedure

### 1. Parse and validate

- Normalize `url` (must start with `http(s)://`).
- Sanitize `name` against `^[a-z0-9][a-z0-9_-]{0,63}$`. If invalid, suggest
  a corrected form and stop.
- `--include` / `--exclude` globs are opaque strings — pass through to the
  harvester.

### 2. Prepare output

- Ensure `packages/docs/<name>/` exists (`mkdir -p`).
- If it exists and `--resume=false`, confirm destructive overwrite with the
  user before proceeding.

### 3. Delegate to `link-harvester`

Invoke the `link-harvester` subagent (Claude Code will resolve it from
`packages/agents/claude/link-harvester.md`) with a single structured
message containing:

```yaml
url: <url>
name: <name>
nav_selector: <--nav-selector or null>
include: <--include or null>
exclude: <--exclude or null>
strip_query: true
resume: <--resume>
max_unfold_passes: 6
```

Wait for it to return. Expect a summary with `harvested`, `new`,
`orphaned`, `output`. If the harvester reports failure (nav not detected,
Playwright MCP unavailable), stop and bubble the actionable message up.

### 4. Delegate to `markdown-converter`

Invoke the `markdown-converter` subagent with:

```yaml
name: <name>
content_selector: <--content-selector or null>
rate_limit_ms: <--rate-limit-ms>
max_errors: <--max-errors>
video_mode: <--video-mode>
```

Let it run to completion. It streams progress via its own logs; the skill
only needs the final summary (`converted`, `skipped_unchanged`, `errors`).

### 5. Update `lock.yaml`

At project root, read `lock.yaml` (create if missing with `{}`). Under
`docs.<name>`, write:

```yaml
docs:
  <name>:
    source: <url>
    version: <YYYY-MM-DD>          # today's UTC date
    count: <number of done entries in links.json>
    hash: "sha256:<hex>"
```

The `hash` is `sha256` over the sorted concatenation of each converted
file's SHA-256 (so it is deterministic and catches any file body change).
Compute via `Bash`:

```sh
find packages/docs/<name> -name '*.md' -type f -print0 \
  | sort -z \
  | xargs -0 shasum -a 256 \
  | awk '{print $1}' \
  | shasum -a 256 \
  | awk '{print "sha256:" $1}'
```

### 6. Report to the user

A short summary that fits on screen:

```
docs-to-markdown: <name>
  source:    <url>
  harvested: <N>
  converted: <M> (errors: <E>)
  output:    packages/docs/<name>/
  lock:      lock.yaml updated (hash sha256:...)
```

## Error handling

| Condition                          | Action                                                    |
| ---------------------------------- | --------------------------------------------------------- |
| Playwright MCP missing             | Stop. Suggest enabling the Playwright MCP server.         |
| Harvester: nav not detected        | Stop. Ask user for `--nav-selector`. No partial writes.   |
| Converter: many pages fail content | Ask for `--content-selector` and offer to rerun pending.  |
| Single-page failures               | Continue; they land in `errors.json`. Surface the count.  |
| `lock.yaml` write race             | Read-modify-write with a file lock (`flock` when present);|
|                                    | on conflict, retry once then abort with a clear message.  |

## Notes

- Never edit the harvester/converter agent files from this skill — they are
  built artifacts under `packages/agents/`. Source lives in `subagents/`.
- Do not attempt to parallelize conversion in v1. Sequential is a goal, not
  a limitation — it preserves rate-limit politeness and determinism.
- This skill's output is consumed by downstream grep/RAG; keep the
  Markdown conventions in the converter agent stable over time.
