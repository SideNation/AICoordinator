# aico CLI

**Claude Code · Codex · Kilo · opencode** 환경에 플러그인(agents·skills·docs·rules·hooks·플랫폼 설정)을 설치·업데이트하는 패키지 관리 도구.

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
export PATH="$PATH:/path/to/AICoordinator/cli/bin"
```

---

## 개념 — 플러그인

설치 단위는 **플러그인**입니다. 플러그인은 `packages/plugins/<source>/` 폴더 하나로, 다음을 담을 수 있습니다.

```
packages/plugins/<source>/
  agents/         # 통합 포맷 .md (플랫폼별로 렌더됨)
  skills/         # <skill>/SKILL.md
  docs/           # 문서 트리
  rules/          # 규칙 .md (claude 전용)
  hooks/          # 훅 스크립트 (claude 전용)
  .claude/        # claude 사이드카 (settings.json 등)
  .codex/         # codex 사이드카 (config.project.toml 등)
  _init/          # 프로젝트 스캐폴드 (init-tree로 워크스페이스에 복사)
  init-tree.yaml  # _init 매핑
  _meta.meta      # 버전(SemVer) + 변경이력 — 설치되지 않음
```

루트 `manifest.yaml`이 플러그인 정의와 버전의 **단일 소스**입니다.

```yaml
# <clone_dir>/manifest.yaml
plugins:
  unity:
    source: unity          # packages/plugins/ 아래 폴더명
    alias: [unity3d]        # CLI에서 받아들일 별칭
    chain: [csharp]         # 함께 설치할 플러그인
    version: 1.0.0          # 각 플러그인 _meta.meta SemVer와 동기화
  csharp:
    source: csharp
    alias: ["c#"]
    version: 1.0.0
  claude-code-game-studio:
    source: claude-code-game-studio
    alias: [ccgs, game-studio]
    target: [claude, codex] # 이 타겟에만 설치 (CLI가 --target all이어도 제한)
    version: 1.1.0

docs:                       # 원격 문서(별도 동기화, 플러그인과 무관)
  onejs:
    source: https://v3.onejs.com/docs/quickstart
    version: 2026-04-20
```

- `target`이 있으면 해당 플랫폼에만 설치됩니다 — `--target all`을 줘도 제한이 우선합니다.
- `chain`에 적힌 플러그인은 자동으로 함께 설치됩니다.

---

## `aico init` — 패키지 저장소 초기화

원하는 폴더로 이동한 뒤 한 번 실행합니다. `PACKAGE_GIT_URL`을 읽어 **현재 폴더 안에** 패키지 저장소를 git clone하고 `~/.aico/.aicorc`에 경로를 기록합니다.

### 동작 순서

1. 실행 폴더의 `.env` 파일에서 `PACKAGE_GIT_URL` 로드.
2. 없으면 환경변수, 그것도 없으면 대화형으로 URL 입력.
3. `<cwd>/<repo-name>/`가 없으면 `git clone`, 이미 있으면 `git pull --ff-only`.
    - 디렉터리 이름은 git URL 끝 세그먼트에서 `.git` 접미사를 제거한 값입니다. 예: `git@github.com:acme/aico-packages.git` → `./aico-packages/`.
4. `~/.aico/.aicorc` 에 `init_dir`, `clone_dir`, `git_url`, 그리고 플랫폼별 모델 매핑 기본값(`models`) 저장.

```bash
cd ~/work
echo "PACKAGE_GIT_URL=git@github.com:acme/aico-packages.git" > .env
aico init
# → ~/work/aico-packages/ 에 clone

# 또는 환경변수 / 대화형
PACKAGE_GIT_URL=https://github.com/acme/aico-packages.git aico init
aico init   # → PACKAGE_GIT_URL (git clone URL): _
```

생성되는 파일:
```
<init 실행 폴더>/
  <repo-name>/            # git clone된 패키지 저장소
    manifest.yaml         # 플러그인·docs 정의와 버전 (단일 소스)
    packages/plugins/     # 플러그인 폴더들
  .env                    # (선택) PACKAGE_GIT_URL 저장
~/.aico/
  .aicorc                 # YAML: init_dir, clone_dir, git_url, models
  .lock                   # YAML: install 기록 (install 실행 시 생성)
```

### `models` — 티어별 모델 오버라이드

agents는 공통 티어(`high`/`medium`/`low`)로 작성하고, 렌더 시 플랫폼별 모델 id로 매핑됩니다. `.aicorc`의 `models`로 매핑을 덮어쓸 수 있습니다(없으면 빌트인 기본값).

```yaml
# ~/.aico/.aicorc
models:
  claude:   { high: opus, medium: sonnet, low: haiku }
  codex:    { high: gpt-5.5, medium: gpt-5.3-codex, low: gpt-5.4-mini }
  opencode: { high: openai/gpt-5.5, medium: openai/gpt-5.3-codex, low: openai/gpt-5.4-mini }
  kilo:     { high: openai/gpt-5.5, medium: openai/gpt-5.3-codex, low: openai/gpt-5.4-mini }
```

effort는 `low`/`medium`/`high`/`xhigh`/`max`를 받으며, `max`는 claude 전용이고 다른 플랫폼에서는 `xhigh`로 치환됩니다.

---

## `aico install` — 플러그인 설치

```bash
aico install <plugin>...      # 이름 또는 별칭으로 설치 (+ chain 동반)
aico install --all            # manifest의 모든 플러그인 설치
```

인자도 `--all`도 없으면 **설치 가능한 플러그인 목록**을 출력하고 종료합니다.

```bash
aico install
# 플러그인 이름을 지정하세요.  예) aico install <name>   또는   aico install --all
# 설치 가능한 플러그인:
#   claude-code-game-studio  1.1.0  ccgs, game-studio
#   csharp                   1.0.0  c#
#   unity                    1.0.0  unity3d
#   ...
```

### 설치 동작 (플러그인별)

| 항목 | 동작 |
|---|---|
| `agents/` | 타겟마다 플랫폼 포맷으로 **렌더링한 실제 파일** 생성 (claude/opencode/kilo `.md`, codex `.toml`) |
| `skills/` | `.claude/skills/`에 복사 + 비-claude 타겟이 있으면 `.agents/skills` 브리지 링크 1개 |
| `docs/` | `.claude/docs/`에 복사 + 비-claude 타겟마다 `<platform>/docs` → `.claude/docs` 링크 |
| `rules/`, `hooks/` | **claude 타겟에만** `.claude/rules/`, `.claude/hooks/`로 복사 |
| `.claude/`, `.codex/` 사이드카 | 타겟 설치 시 해당 설정 폴더로 복사. `settings.json`·`config.toml`은 **딥 병합**(아래) |
| `_init/` + `init-tree.yaml` | 프로젝트 스캐폴드를 워크스페이스에 **최초 1회만** 복사 ([aico init-tree](#aico-init-tree--프로젝트-스캐폴드-재적용)) |
| `_meta.meta` | 복사하지 않음 (버전 추적용) |

### 설치 경로

| 종류 | scope=project | scope=user (`-g`) |
|---|---|---|
| claude agent | `.claude/agents/<n>.md` | `~/.claude/agents/` |
| codex agent | `.codex/agents/<n>.toml` | `~/.codex/agents/` |
| kilo agent | `.kilo/agents/<n>.md` | `~/.config/kilo/agents/` |
| opencode agent | `.opencode/agents/<n>.md` | `~/.config/opencode/agents/` |
| skills | `.claude/skills/` (+ `.agents/skills` 브리지) | `~/.claude/skills/` |
| docs | `.claude/docs/` (+ 비-claude 링크) | `~/.claude/docs/` |
| rules / hooks | `.claude/rules/`, `.claude/hooks/` | `~/.claude/...` |

### 설정 딥 병합 (없는 것만 추가)

플러그인 `.claude/settings.json` → 프로젝트 `.claude/settings.json`(JSON), `.codex/config.project.toml` → `.codex/config.toml`(TOML)로 병합됩니다.

- 객체는 재귀 병합, 배열(`permissions.allow` 등)은 **빠진 항목만** 합집합으로 추가.
- **기존 값은 절대 덮어쓰지 않습니다.** 대상 파일이 없으면 통째로 복사.

### install 플래그

| 플래그 | 기본값 | 설명 |
|---|---|---|
| `--all` | `false` | manifest의 모든 플러그인 설치 |
| `-g`, `--global` | `false` | 홈 디렉터리(`~/.claude`, `~/.codex`, `~/.config/...`)에 설치 |
| `--target` | `all` | 타겟 플랫폼: `claude`/`cl`, `codex`/`co`, `kilo`/`ki`, `opencode`/`op`, `all` |
| `--src` | (auto) | 패키지 소스 디렉터리 오버라이드. 기본은 `.aicorc`의 `clone_dir/packages` |

```bash
aico install unity                 # unity + chain(csharp)
aico install ccgs                  # 별칭 → claude-code-game-studio (target=claude,codex)
aico install --all                 # 전체
aico install csharp --target codex # codex로만
aico install -g --all              # 사용자 환경에 전체
```

---

## `aico init-tree` — 프로젝트 스캐폴드 재적용

`install`은 플러그인의 `_init/` 스캐폴드를 워크스페이스에 **최초 1회만** 복사합니다(이후 `update`는 건너뜀, 기존 파일은 덮어쓰지 않음). 스캐폴드를 다시 깔거나 강제로 덮어쓰려면 이 명령을 씁니다.

```bash
aico init-tree <plugin>...   # 지정 플러그인의 _init를 현재 폴더에 강제 복사(덮어쓰기)
aico init-tree --all
```

`init-tree.yaml`은 `_init/` 항목의 복사 대상을 매핑합니다.

```yaml
# packages/plugins/<source>/init-tree.yaml
workspace:        # 프로젝트 루트로 복사
  - src
  - design
  - CLAUDE.md
.claude:          # 프로젝트 .claude/ 로 복사
  - statusline.sh
```

| 플래그 | 기본값 | 설명 |
|---|---|---|
| `--all` | `false` | manifest의 모든 플러그인 스캐폴드 적용 |
| `--src` | (auto) | 패키지 소스 디렉터리 오버라이드 |

---

## `aico update` — manifest 버전과 동기화

1. `.aicorc`의 `clone_dir`에서 `git pull --ff-only` (실패해도 계속 진행).
2. `<clone_dir>/manifest.yaml`을 로드 (없으면 에러).
3. `.lock`에 추적된 각 설치 지점에 대해:
   - **버전이 달라진 플러그인만 재설치**(자산만 — `_init` 스캐폴드는 재적용하지 않음).
   - 버전이 같으면 건너뜀.
   - `.lock`에 있지만 manifest에서 사라진 플러그인은 **삭제 여부를 질문**(`y/N`).
4. `.lock`의 각 플러그인 버전을 manifest 값으로 갱신.

새 플러그인을 추가하려면 `aico install <plugin>`을 사용하세요. `update`는 이미 설치된 플러그인만 동기화합니다.

```bash
aico update          # 현재 폴더가 설치되어 있다면 현재 폴더만
aico update -g       # 사용자 환경만
aico update --all    # .lock의 모든 설치 (사라진 project 폴더는 .lock에서 제거)
```

| 플래그 | 기본값 | 설명 |
|---|---|---|
| `--all` | `false` | 추적 중인 모든 설치를 업데이트하고 사라진 project 폴더 정리 |
| `-g`, `--global` | `false` | user 스코프(홈 디렉터리) 설치만 업데이트 |

---

## `aico list` — 설치 상태 조회

`.lock`의 설치 항목, manifest와의 비교를 보여줍니다. 별칭 `aico ls`도 됩니다. 각 플러그인은 **설치 유무**와 **마지막 설치/업데이트 날짜·시각**(`install`·`update` 실행 시 `.lock`에 기록)이 함께 표시됩니다.

```bash
aico list          # 현재 폴더(또는 -g 시 홈)에 설치된 플러그인
aico list -a       # .lock의 모든 설치 위치
aico list -v       # manifest에 있지만 미설치인 플러그인
aico list -m       # manifest 전체 + 설치 상태 (✓ 설치됨 / ○ 미설치)
```

### 출력 예

```
Install: /Users/me/myproject (scope=project, target=claude,codex,kilo,opencode)
  package version: 1a3a361...
Plugins:
  csharp  1.0.0  2026-06-15 03:31:42
  unity   1.0.0  2026-06-15 03:31:50
```

`aico list -m` — ✓ 설치됨 / ○ 미설치, 설치된 항목에는 마지막 설치/업데이트 시각 표시:
```
Plugins:
  ○ claude-code-game-studio  1.1.0  not installed
  ✓ csharp                   1.0.0  up to date (2026-06-15 03:31:42)
  ✓ unity                    1.0.0  installed 0.9.0 → update available (2026-06-15 03:31:50)
```

| 플래그 | 단축 | 설명 |
|---|---|---|
| `--global` | `-g` | user 스코프(홈 디렉터리) 기준 |
| `--all` | `-a` | `.lock`의 모든 설치 표시 |
| `--available` | `-v` | manifest에 있지만 미설치인 플러그인 |
| `--manifest` | `-m` | manifest 전체 + 설치 상태 |

`-v`와 `-m`은 같이 쓸 수 없습니다.

---

## `aico rm` — 설치된 플러그인 삭제

플러그인 단위로 디스크 자산과 `.lock` 기록을 함께 제거합니다. 어떤 파일을 지울지는 패키지 소스의 플러그인 폴더를 다시 읽어 판단하므로, 패키지 소스가 있어야 합니다.

```bash
aico rm csharp               # 이름 또는 별칭
aico rm 'unity*'             # glob (설치된 플러그인 이름 대상)
aico rm csharp unity         # 여러 개
aico rm -g ccgs              # 사용자 환경에서 삭제
```

### rm 동작
- 플러그인의 agents(전 플랫폼 렌더 파일), skills(이름별 디렉터리), docs/rules/hooks(플러그인이 기여한 파일)를 제거.
- 매칭되는 설치 플러그인이 없으면 경고만 표시하고 종료(에러 X).
- 디스크에 이미 없는 파일은 무시하고 `.lock`만 정리합니다.

| 플래그 | 기본값 | 설명 |
|---|---|---|
| `-g`, `--global` | `false` | user 스코프(홈 디렉터리)에서 삭제 |
| `--src` | (auto) | 패키지 소스 디렉터리 오버라이드 |

---

## `aico codex` — `.agents` → `.claude` 링크

비-Claude 도구가 Claude 자산을 재사용하도록 `.agents`를 `.claude`로 가리키는 링크를 만듭니다(macOS/Linux 심볼릭 링크, Windows 디렉터리 정션).

```bash
aico codex          # 현재 폴더에 .agents → .claude
aico codex -g       # 홈에 ~/.agents → ~/.claude
aico codex -f       # 기존 링크 교체
```

---

## 디렉터리 구조

```
cli/
├── bin/                        # 빌드 결과물
├── src/
│   ├── main.go
│   ├── cmd/
│   │   ├── root.go             # 루트 커맨드 + .aicorc models 주입
│   │   ├── init.go             # init (.env 로드, git clone, .aicorc 저장)
│   │   ├── install.go          # install (플러그인 설치 + 공용 헬퍼)
│   │   ├── init_tree.go        # init-tree (_init 스캐폴드 복사)
│   │   ├── update.go           # update (manifest 버전 동기화)
│   │   ├── list.go             # list (설치 현황/manifest 비교)
│   │   ├── rm.go               # rm (플러그인 삭제)
│   │   ├── merge.go            # settings.json/config.toml 딥 병합
│   │   ├── link.go             # 심볼릭/정션 링크 프리미티브
│   │   └── codex.go            # .agents → .claude 링크
│   ├── agent/                  # 통합 agent 파싱 + 플랫폼별 렌더
│   │   ├── source.go  validate.go  platform.go
│   │   ├── render_claude.go  render_codex.go  render_kilo.go  render_opencode.go
│   │   └── render_common.go    # 모델/색상 매핑, YAML/TOML 이미터
│   └── config/
│       └── config.go           # .aicorc, .lock, manifest 로드·저장
├── build.sh
├── go.mod
└── go.sum
```
