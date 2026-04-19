# PRD v1 — agent splitter CLI (aico)

## Context

사용자는 agent를 한 번만 작성하고 **Claude Code**와 **opencode** 두 도구용 파일로 자동 분리하기를 원한다. 본문(Markdown body)은 거의 동일하지만 frontmatter 스펙이 서로 다르고(필드명·값 모양·허용값이 다름), 각 도구에만 있는 필드도 많다. 수동 동기화는 드리프트와 오타의 원인이 된다.

이 CLI(`aico`)는 `subagents/` 아래 **단일 소스 마크다운**을 입력받아 `packages/agents/claude/<name>.md` 와 `packages/agents/opencode/<name>.md` 로 변환·출력한다. 공유 가능한 필드는 공유하고, 도구별 전용 필드는 네임스페이스 블록으로 분리 작성한다. `useonly` 예약 필드로 한쪽 대상만 빌드할 수도 있다.

조사 결과(요약):
- **공통 의미 필드**: `description`, `model`, `tools`, `color` — 단 값 표현이 다름(모델 alias vs `provider/model-id`, tools CSV vs `{name: bool}`, color 이름 vs hex).
- **Claude 전용**: `name`, `disallowedTools`, `maxTurns`, `skills`, `mcpServers`, `hooks`, `memory`, `background`, `effort`, `isolation`, `initialPrompt`, `permissionMode`.
- **opencode 전용**: `mode`, `temperature`, `top_p`, `steps`, `disable`, `hidden`, `permission`(중첩).
- 정체성: Claude는 `name:` 필드, opencode는 파일명이 id. 한쪽에서 다른쪽을 유도해야 함.

## 목표 / 비목표

**목표**
1. 단일 소스 `.md` → Claude용 / opencode용 2파일 생성.
2. 공유 필드는 top-level에, 도구 전용 필드는 `claude:` / `opencode:` 네임스페이스 블록에 작성.
3. 값 모양 변환(model alias ↔ provider/model-id, tools CSV ↔ map, color 이름 ↔ hex) 지원.
4. 대상에 존재하지 않는 필드는 경고와 함께 드롭.
5. Body(프론트매터 이후 본문)는 그대로 복사.
6. `aico` Go 앱에 서브커맨드로 통합(`aico agent split`, `aico agent build`).
7. `useonly` 예약 필드로 빌드 대상 제한(미지정/빈값=둘 다, `claude`/`opencode` 단일 지정 가능).

**비목표**
- 역변환(2파일 → 단일 소스) 이번 버전에서 지원하지 않음.
- `lock.yaml` 버전 해시 계산 로직은 별도 PRD.
- MCP 서버/훅 스키마 validate는 pass-through(내용 검사하지 않음).

## 입력 소스 포맷

`subagents/<name>.md`:

```markdown
---
# 공유 코어 (양쪽 모두 해석)
name: code-reviewer              # 필수. 파일명과 동일 권장
description: Reviews PRs for ... # 필수
model: sonnet                    # alias 또는 provider/model-id
tools: [Read, Grep, Bash]        # 통일된 배열 형태
color: blue                      # 이름 기반 (팔레트 매핑 테이블로 hex 유도)

# 빌드 대상 제한 (예약 필드)
# useonly: claude        # → claude 패키지만 생성
# useonly: opencode      # → opencode 패키지만 생성
# (미지정 또는 빈값 / 'both' / 'all' → 둘 다 생성)

# Claude 전용 오버라이드
claude:
  permissionMode: acceptEdits
  disallowedTools: [Write]
  maxTurns: 20
  skills: [review]
  effort: high

# opencode 전용 오버라이드
opencode:
  mode: subagent
  temperature: 0.2
  steps: 40
  permission:
    edit: ask
    bash: allow
---

Body markdown content here...
```

공유 코어의 필드가 네임스페이스 블록에도 있으면 **네임스페이스 값이 우선**.

### `useonly` 필드 규칙

| 값 | 동작 |
|---|---|
| 필드 없음 / `""` / `both` / `all` | claude + opencode 둘 다 생성 |
| `claude` | `packages/agents/claude/<name>.md` 만 생성 |
| `opencode` | `packages/agents/opencode/<name>.md` 만 생성 |
| 기타 값 | 검증 에러 |

`useonly`는 빌드 시 제외 대상을 **출력하지 않을** 뿐만 아니라, 기존에 남아있는 반대편 산출물이 있으면 경고를 남긴다(자동 삭제는 하지 않음; `--prune` 플래그 시에만 삭제).

## 출력

### `packages/agents/claude/<name>.md`
```yaml
---
name: code-reviewer
description: ...
model: sonnet
tools: Read, Grep, Bash         # CSV로 변환
color: blue                      # 이름 유지
permissionMode: acceptEdits
disallowedTools: Write
maxTurns: 20
skills: [review]
effort: high
---
<body 동일>
```

### `packages/agents/opencode/<name>.md`
```yaml
---
description: ...
mode: subagent
model: anthropic/claude-sonnet-4-6   # alias → provider/model-id 매핑
tools:                                # 배열 → {name: true}
  read: true
  grep: true
  bash: true
color: "#3b82f6"                      # 이름 → hex 매핑
temperature: 0.2
steps: 40
permission:
  edit: ask
  bash: allow
---
<body 동일>
```
(opencode는 파일명이 id이므로 `name` 필드 생략. `useonly` 예약 필드는 두 출력 모두에서 제거.)

## 변환 규칙

| 소스 | → Claude | → opencode |
|---|---|---|
| `name` | 그대로 | 출력 파일명에 반영, 필드는 생략 |
| `description` | 그대로 | 그대로 |
| `useonly` | 출력에서 제거 (빌드 대상 결정에만 사용) | 동일 |
| `model: sonnet` | `sonnet` | `anthropic/claude-sonnet-4-6` (매핑표) |
| `model: opus` | `opus` | `anthropic/claude-opus-4-7` |
| `model: haiku` | `haiku` | `anthropic/claude-haiku-4-5-20251001` |
| `model: provider/id` | 역매핑표로 alias 유도, 없으면 그대로 | 그대로 |
| `tools: [A, B]` | `"A, B"` (CSV) | `{a: true, b: true}` (lowercase) |
| `color: blue` | `blue` | `#3b82f6` (팔레트) |
| `color: "#hex"` | 가장 가까운 이름 또는 경고 후 드롭 | 그대로 |
| `claude.*` | 그대로 병합 | 드롭 |
| `opencode.*` | 드롭 | 그대로 병합 |

모델·색상 매핑표는 `cli/src/mapping/` 아래 yaml로 두어 수정 용이하게.

## CLI 설계 (Go)

바이너리: `aico` (`cli/bin/aico`)

### 서브커맨드
- `aico agent split <source.md> [--out-dir packages/agents]`
  - 하나의 파일을 두 대상으로 분리 작성 (`useonly` 준수).
- `aico agent build [--src subagents] [--out packages/agents]`
  - `subagents/` 전체를 일괄 변환. 변경된 파일만 재생성.
- `aico agent validate <source.md>`
  - 스키마·필수필드·허용값(`useonly` 포함) 검증, 쓰기 없음.

### 플래그
- `--dry-run`: 출력 대신 stdout에 렌더 결과 표시.
- `--force`: 기존 출력 덮어쓰기(기본은 mtime 비교).
- `--strict`: 매핑 불가 값이면 실패(기본은 경고 후 드롭).
- `--prune`: `useonly`로 제외된 대상의 기존 산출물을 삭제.
- `--only claude|opencode`: CLI 차원에서 대상 강제(소스의 `useonly`보다 우선).

### 종료 코드
`0` 성공, `1` 검증 실패, `2` IO 오류, `3` 매핑 실패(--strict).

## 모듈 구성

```
cli/src/
├── cmd/
│   ├── root.go
│   └── agent.go               # split/build/validate 래퍼
├── agent/
│   ├── parser.go              # frontmatter + body 파싱 (yaml.v3)
│   ├── model.go               # Source, ClaudeOut, OpencodeOut, UseOnly enum
│   ├── transform_claude.go
│   ├── transform_opencode.go
│   └── writer.go
├── mapping/
│   ├── models.yaml            # alias ↔ provider/id
│   └── colors.yaml            # name ↔ hex
└── main.go
```

## 엣지 케이스

1. `claude:`/`opencode:` 블록에 공유 필드와 충돌 → 네임스페이스 우선, 경고 로그.
2. 소스에 Claude 전용 필드가 top-level에 있으면 → 검증 에러(네임스페이스 블록으로 이동 요구).
3. 알 수 없는 모델 alias → `--strict`에서 실패, 아니면 원문 pass-through + 경고.
4. Body 없는 파일 → 허용(frontmatter-only).
5. 출력 디렉터리 없음 → 생성.
6. 같은 `name`이 두 소스에 존재 → 빌드 시 에러.
7. `useonly: opencode` 인데 `claude:` 오버라이드 블록만 있음 → 경고(사용되지 않는 블록).
8. `--only` 와 `useonly` 충돌 → `--only` 우선, 정보 로그 출력.

## 검증 방법

1. `cli/test/fixtures/` 에 샘플 4종(최소/전체/엣지/useonly-단일) 추가.
2. `go test ./agent/...` 로 변환 스냅샷 테스트.
3. 샘플 출력물을 실제 Claude Code(`.claude/agents/`)·opencode(`.opencode/agent/`)에 수동 투입하여 둘 다 정상 로드되는지 확인.
4. `aico agent validate` 가 누락된 필수 필드(`name`, `description`)와 잘못된 `useonly` 값을 정확히 지적하는지 확인.
5. `useonly: claude` 소스를 빌드했을 때 opencode 쪽 파일이 생성되지 않는지 확인.

## 구현할 주요 파일

- `cli/prd/prd_v1.md` — 본 문서 최종본
- `cli/src/cmd/agent.go` — CLI 서브커맨드
- `cli/src/agent/parser.go`, `model.go`, `transform_*.go`, `writer.go`
- `cli/src/mapping/models.yaml`, `colors.yaml`
- `cli/test/fixtures/*.md`

## 향후 작업(별도 PRD)

- `lock.yaml` 해시 생성 및 드리프트 감지.
- `skills` / `docs` 대상 동일한 split 파이프라인.
- 역변환(두 파일 → 단일 소스) 마이그레이션 헬퍼.
