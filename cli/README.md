# aico CLI

**Claude Code**와 **opencode** 환경에 agents, skills, docs를 설치·업데이트하는 패키지 관리 도구.

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
  <repo-name>/            # git clone된 패키지 저장소
    manifest.yaml         # 설치 대상과 버전을 선언 (수동 편집)
    packages/             # agents/ skills/ docs/ 하위 디렉터리 포함
  .env                    # (선택) PACKAGE_GIT_URL 저장
~/.aico/
  .aicorc                 # YAML: init_dir, clone_dir, git_url
  .lock                   # YAML: install 기록 (install 실행 시 생성)
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

### manifest.yaml — 설치 대상 선언

`install`과 `update`는 모두 `<clone_dir>/manifest.yaml`을 **권위 있는 버전 기준**으로 사용합니다. manifest에 선언된 항목만 설치·추적되며, 각 항목의 `version`이 `.lock`에 기록됩니다.

```yaml
# <clone_dir>/manifest.yaml
agents:
  link-harvester:
    version: 2026-04-19
  markdown-converter:
    version: 2026-04-19

skills:
  docs-to-markdown:
    version: 2026-04-19

docs:
  onejs:
    version: 2026-04-20
  backnd-base:
    version: 2026-04-19
```

- manifest에 없는 항목은 설치되지 않습니다 (경고 후 스킵).
- manifest의 `version`을 바꾸면 다음 `aico update`가 해당 항목만 재설치합니다.
- manifest가 없으면 `install`/`update`는 에러로 종료됩니다.

### 기본 설치 범위

| 항목 | 기본 | 비고 |
|---|---|---|
| agents (Claude) | ✅ | manifest에 선언된 agent만. `--target opencode` / `--target all`로 대상 전환 |
| agents (opencode) | ❌ | `--target` 지정 필요 |
| skills | ✅ | manifest에 선언된 skill 전부 |
| docs | ❌ | `--docs` 플래그로 활성화. manifest에 선언된 것 중 선택 설치 |

### `--docs` 동작

- 플래그 없음 → docs 미설치
- `--docs` (값 없음) → manifest에 선언된 **모든** docs 설치
- `--docs=이름1,이름2` → 지정한 docs만 설치 (manifest에 선언되어 있어야 함)
- **이미 설치된 docs는 자동 스킵**. 버전이 바뀌었다면 `aico update`를 사용하세요.
- 설치된 docs 이름과 버전은 `~/.aico/.lock`에 기록됩니다.

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
aico install -g --docs

# 사용자 환경에 전체 설치 (Claude + opencode + 모든 docs)
aico install -g --target all --docs
```

### install 플래그

| 플래그 | 기본값 | 설명 |
|---|---|---|
| `-g`, `--global` | `false` | 지정하면 홈 디렉터리(`~/.claude`, `~/.config/opencode`)에 설치. 생략 시 현재 프로젝트 디렉터리 |
| `--target` | `claude` | agent 대상: `claude`, `opencode`, `all` |
| `--docs` | (unset) | 지정하면 docs 설치. 값 없으면 전부, `a,b` 형식으로 선택 설치. 이미 설치된 것은 스킵 |
| `--src` | (auto) | 패키지 소스 디렉터리 오버라이드. 기본은 `.aicorc`의 `clone_dir/packages` |

---

## `aico update` — manifest 버전과 동기화

1. `.aicorc`의 `clone_dir`에서 `git pull --ff-only` (실패해도 계속 진행).
2. `<clone_dir>/manifest.yaml`을 로드 (없으면 에러).
3. `.lock`에 추적된 각 설치 지점에 대해:
   - **버전이 달라진 항목만 재설치** (skills/docs는 기존 디렉터리를 지우고 교체, agents는 `.md` 파일 덮어쓰기).
   - 버전이 같으면 건너뜀.
   - `.lock`에 있지만 manifest에서 사라진 항목은 **삭제 여부를 사용자에게 질문** (`y/N`).
4. `.lock`의 각 항목 버전을 manifest 값으로 갱신.

새 항목을 추가로 설치하려면 `aico install --docs=<이름>` 등을 사용하세요. `update`는 이미 설치된 항목만 동기화합니다.

```bash
# 기본: 현재 폴더가 설치되어 있다면 현재 폴더만 업데이트
aico update

# 사용자 환경만 업데이트
aico update -g

# .lock에 기록된 모든 설치를 업데이트 (존재하지 않는 project 폴더는 .lock에서 제거)
aico update --all
```

### update 플래그

| 플래그 | 기본값 | 설명 |
|---|---|---|
| `--all` | `false` | 추적 중인 모든 설치를 업데이트하고, 사라진 project 폴더는 `.lock`에서 제거 |
| `-g`, `--global` | `false` | user 스코프(홈 디렉터리) 설치만 업데이트 |

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
│   │   ├── install.go          # install 커맨드
│   │   ├── init.go             # init 커맨드 (.env 로드, git clone, .aicorc 저장)
│   │   └── update.go           # update 커맨드 (git pull + 재설치)
│   └── config/
│       └── config.go           # ~/.aico/.aicorc, .lock 로드·저장
├── build.sh                    # 멀티 플랫폼 빌드 스크립트
├── go.mod
└── go.sum
```
