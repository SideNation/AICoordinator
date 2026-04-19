# aico CLI

단일 소스 에이전트 마크다운 파일을 **Claude Code**용과 **opencode**용 파일로 자동 분리·변환합니다.

---

## 빌드

### 요구사항

- Go 1.21 이상
- `cli/` 디렉터리에서 실행

### 빌드 스크립트

```bash
cd cli
bash build.sh
```

다음 바이너리가 `cli/bin/` 에 생성됩니다.

| 파일 | 플랫폼 |
|---|---|
| `aico-darwin-arm64` | macOS Apple Silicon |
| `aico-darwin-amd64` | macOS Intel |
| `aico-windows-amd64.exe` | Windows 64-bit |
| `aico` | 현재 플랫폼의 네이티브 복사본 |

### 수동 빌드

```bash
cd cli

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o bin/aico-darwin-arm64 ./src

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -o bin/aico-darwin-amd64 ./src

# Windows
GOOS=windows GOARCH=amd64 go build -o bin/aico-windows-amd64.exe ./src
```

### PATH 등록 (선택)

```bash
# macOS — 셸 프로필에 추가
export PATH="$PATH:/path/to/AISkills/cli/bin"
```

---

## 소스 포맷

`agents/<name>.md` 파일에 단일 소스로 작성합니다.

```markdown
---
name: code-reviewer
description: Reviews PRs for correctness and style
model: sonnet            # alias 또는 anthropic/provider-id
tools: [Read, Grep, Bash]
color: blue              # 이름(blue/red/…) 또는 #hex
useonly: both            # 생략·both·all = 양쪽, claude = Claude만, opencode = opencode만

# Claude Code 전용 오버라이드
claude:
  permissionMode: acceptEdits
  disallowedTools: [Write]
  maxTurns: 20
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

에이전트 본문을 여기에 작성합니다.
```

### 필수 필드

| 필드 | 설명 |
|---|---|
| `name` | 에이전트 식별자 (파일명 권장) |
| `description` | 에이전트 설명 |

### 공유 필드 변환 규칙

| 소스 | Claude 출력 | opencode 출력 |
|---|---|---|
| `model: sonnet` | `sonnet` | `anthropic/claude-sonnet-4-6` |
| `model: opus` | `opus` | `anthropic/claude-opus-4-7` |
| `model: haiku` | `haiku` | `anthropic/claude-haiku-4-5-20251001` |
| `tools: [Read, Grep]` | `Read, Grep` (CSV) | `{read: true, grep: true}` (맵) |
| `color: blue` | `blue` | `#3b82f6` |
| `color: "#3b82f6"` | `blue` (역매핑) | `#3b82f6` |

---

## 커맨드

### `aico agent split` — 단일 파일 분리

```bash
aico agent split <source.md> [플래그]
```

`agents/<name>.md` 한 파일을 읽어 두 대상으로 씁니다.

```bash
# 기본 (--out-dir packages/agents 기준)
aico agent split agents/code-reviewer.md

# 출력 디렉터리 지정
aico agent split agents/code-reviewer.md --out-dir packages/agents

# 결과를 파일에 쓰지 않고 stdout에 출력
aico agent split agents/code-reviewer.md --dry-run

# 기존 파일 강제 덮어쓰기
aico agent split agents/code-reviewer.md --force

# Claude 파일만 생성
aico agent split agents/code-reviewer.md --only claude

# opencode 파일만 생성
aico agent split agents/code-reviewer.md --only opencode

# 매핑 불가 값 발견 시 경고 대신 실패
aico agent split agents/code-reviewer.md --strict
```

### `aico agent build` — 전체 일괄 변환

```bash
aico agent build [플래그]
```

`agents/` 디렉터리 내 모든 `.md` 파일을 일괄 변환합니다. mtime을 비교해 변경된 파일만 재생성합니다.

```bash
# 기본
aico agent build

# 소스·출력 디렉터리 지정
aico agent build --src agents --out-dir packages/agents

# 모든 파일 강제 재생성
aico agent build --force

# useonly로 제외된 대상 파일이 남아 있으면 삭제
aico agent build --prune

# dry-run: 실제로 쓰지 않고 결과만 출력
aico agent build --dry-run
```

### `aico agent validate` — 유효성 검사

```bash
aico agent validate <source.md>
```

파일을 읽어 필수 필드·허용값을 검사합니다. 파일을 쓰지 않습니다.

```bash
aico agent validate agents/code-reviewer.md
# → 오류 없으면 exit 0, 오류 있으면 목록 출력 후 exit 1
```

### 플래그 요약

| 플래그 | 기본값 | 설명 |
|---|---|---|
| `--out-dir` | `packages/agents` | 출력 루트 디렉터리 |
| `--src` | `agents` | `build` 시 소스 디렉터리 |
| `--dry-run` | `false` | 파일을 쓰지 않고 stdout에 렌더 결과 출력 |
| `--force` | `false` | mtime 비교 없이 강제 덮어쓰기 |
| `--strict` | `false` | 매핑 불가 값 발견 시 실패 (기본은 경고 후 드롭) |
| `--prune` | `false` | useonly 제외 대상의 기존 출력 파일 삭제 |
| `--only` | `""` | `claude` 또는 `opencode` 로 대상 강제 지정 |

---

## 종료 코드

| 코드 | 의미 |
|---|---|
| `0` | 성공 |
| `1` | 검증 실패 (필드 오류, useonly 오류 등) |
| `2` | IO 오류 |
| `3` | 매핑 실패 (`--strict` 모드) |

---

## 테스트

```bash
cd cli
go test ./src/agent/...
```

`cli/test/fixtures/` 아래 샘플 파일을 기반으로 53개 테스트가 실행됩니다.

---

## 디렉터리 구조

```
cli/
├── bin/                        # 빌드 결과물
│   ├── aico                    # 현재 플랫폼 네이티브
│   ├── aico-darwin-arm64
│   ├── aico-darwin-amd64
│   └── aico-windows-amd64.exe
├── src/
│   ├── main.go
│   ├── cmd/
│   │   ├── root.go
│   │   └── agent.go            # split / build / validate 서브커맨드
│   ├── agent/
│   │   ├── model.go            # Source, ClaudeOut, OpencodeOut 구조체
│   │   ├── parser.go           # 프론트매터 파싱, 유효성 검사
│   │   ├── mapping.go          # 모델·색상 매핑 로더
│   │   ├── transform_claude.go
│   │   ├── transform_opencode.go
│   │   └── writer.go           # YAML 렌더링, 파일 쓰기
│   └── mapping/
│       ├── models.yaml         # alias ↔ provider/model-id
│       └── colors.yaml         # 색상 이름 ↔ hex
├── test/
│   └── fixtures/               # 테스트용 샘플 마크다운
├── docs/
│   └── prd_v1.md               # 제품 개발 계획서
├── build.sh                    # 멀티 플랫폼 빌드 스크립트
├── go.mod
└── go.sum
```
