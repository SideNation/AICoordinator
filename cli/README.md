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

---

## `aico init` — 패키지 저장소 초기화

원하는 폴더로 이동한 뒤 한 번 실행합니다. `PACKAGE_GIT_URL`을 읽어 **현재 폴더 안에** 패키지 저장소를 git clone하고 `~/.aico/.aicorc`에 경로를 기록합니다.

### 동작 순서

1. 실행 폴더의 `.env` 파일에서 `PACKAGE_GIT_URL` 로드.
2. 없으면 환경변수, 그것도 없으면 대화형으로 URL 입력.
3. `<cwd>/<repo-name>/`가 없으면 `git clone`, 이미 있으면 `git pull --ff-only`.
    - 디렉터리 이름은 git URL 끝 세그먼트에서 `.git` 접미사를 제거한 값입니다. 예: `git@github.com:acme/aico-packages.git` → `./aico-packages/`.
4. `~/.aico/.aicorc` 에 `init_dir`, `clone_dir`, `git_url` 저장.

```bash
# 패키지 저장소를 둘 폴더로 이동
cd ~/work

# .env에 PACKAGE_GIT_URL을 적어 두고 실행
echo "PACKAGE_GIT_URL=git@github.com:acme/aico-packages.git" > .env
aico init
# → ~/work/aico-packages/ 에 clone

# 또는 환경변수로
PACKAGE_GIT_URL=https://github.com/acme/aico-packages.git aico init

# 아무 값도 없으면 CLI가 URL을 물어봅니다
aico init
# → PACKAGE_GIT_URL (git clone URL): _
```

생성되는 파일:
```
<init 실행 폴더>/
  <repo-name>/     # git clone된 패키지 저장소
  .env             # (선택) PACKAGE_GIT_URL 저장
~/.aico/
  .aicorc          # YAML: init_dir, clone_dir, git_url
  .lock            # YAML: install 기록 (install 실행 시 생성)
```

---

## `aico install` — 환경에 설치

패키지 저장소(`<clone_dir>/packages/`)에서 완성된 파일을 Claude Code·opencode가 읽는 경로에 복사합니다. 설치 정보는 `~/.aico/.lock`에 기록되어 이후 `aico update`가 추적합니다.

### 설치 경로

| 종류 | scope=project | scope=user |
|---|---|---|
| Claude agent | `.claude/agents/` | `~/.claude/agents/` |
| opencode agent | `.opencode/agents/` | `~/.config/opencode/agents/` |
| skills | `.claude/skills/` | `~/.claude/skills/` |
| docs | `.claude/docs/` | `~/.claude/docs/` |

> skills와 docs는 Claude Code와 opencode 모두 `.claude/` 경로에서 로드하므로 Claude 경로에만 복사합니다.

### 기본 설치 범위

| 항목 | 기본 | 비고 |
|---|---|---|
| Claude agent | ✅ | `--target opencode` / `--target all`로 전환·확장 |
| opencode agent | ❌ | `--target` 지정 필요 |
| skills | ✅ | 항상 설치 |
| docs | ❌ | `--docs` 플래그로 활성화 (이름 선택 가능) |

### `--docs` 동작

- 플래그 없음 → docs 미설치
- `--docs` (값 없음) → `packages/docs/` 아래 **모든** docs 설치
- `--docs=이름1,이름2` → 지정한 docs만 설치 (쉼표 구분, 공백 허용)
- **이미 설치된 docs는 자동 스킵**. 다시 내려받으려면 `aico update`를 사용하세요.
- 설치된 docs 이름은 `~/.aico/.lock`에 기록되어 이후 `update`가 같은 목록으로 재설치합니다.

```bash
# 기본: project 스코프, Claude agent + skills
aico install

# 모든 docs 설치
aico install --docs

# 선택한 docs만 설치
aico install --docs=backnd-base
aico install --docs=backnd-base,frontend-guide

# opencode agent도 함께
aico install --target all

# 사용자 환경(~/.claude, ~/.config/opencode)에 docs 포함 설치
aico install --scope user --docs

# 사용자 환경에 전체 설치 (Claude + opencode + 모든 docs)
aico install --scope user --target all --docs
```

### install 플래그

| 플래그 | 기본값 | 설명 |
|---|---|---|
| `--scope` | `project` | `project` = 현재 디렉터리, `user` = 홈 디렉터리 |
| `--target` | `claude` | agent 대상: `claude`, `opencode`, `all` |
| `--docs` | (unset) | 지정하면 docs 설치. 값 없으면 전부, `a,b` 형식으로 선택 설치. 이미 설치된 것은 스킵 |
| `--src` | (auto) | 패키지 소스 디렉터리 오버라이드. 기본은 `.aicorc`의 `clone_dir/packages` |

---

## `aico update` — 최신 패키지로 갱신

1. `.aicorc`의 `clone_dir`에서 `git pull --ff-only` (실패해도 계속 진행).
2. `.lock`의 기록 중 조건에 맞는 설치 지점을 **현재 설치된 구성 그대로** 재설치.
   - agent·skills는 항상 갱신.
   - docs는 `.lock`에 기록된 이름 목록을 통째로 재설치(기존 디렉터리는 깨끗하게 교체).

새 docs를 추가로 설치하려면 `aico install --docs=<이름>`을 사용하세요. `update`는 새 docs를 추가하지 않고, 이미 설치된 항목만 최신 상태로 맞춥니다.

```bash
# 기본: 현재 폴더가 설치되어 있다면 현재 폴더만 업데이트
aico update

# 사용자 환경만 업데이트
aico update --user

# .lock에 기록된 모든 설치를 업데이트 (존재하지 않는 project 폴더는 .lock에서 제거)
aico update --all
```

### update 플래그

| 플래그 | 기본값 | 설명 |
|---|---|---|
| `--all` | `false` | 추적 중인 모든 설치를 업데이트하고, 사라진 project 폴더는 `.lock`에서 제거 |
| `--user` | `false` | user 스코프 설치만 업데이트 |

---

### agent 플래그 요약

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
│   │   ├── agent.go            # split / build / validate 서브커맨드
│   │   ├── install.go          # install 커맨드
│   │   ├── init.go             # init 커맨드 (.env 로드, git clone, .aicorc 저장)
│   │   └── update.go           # update 커맨드 (git pull + 재설치)
│   ├── config/
│   │   └── config.go           # ~/.aico/.aicorc, .lock 로드·저장
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
│   ├── prd_v1.md               # 제품 개발 계획서 v1
│   ├── prd_v2.md               # 제품 개발 계획서 v2 (install 커맨드)
│   └── prd_v3.md               # 제품 개발 계획서 v3 (init / update / .lock)
├── build.sh                    # 멀티 플랫폼 빌드 스크립트
├── go.mod
└── go.sum
```
