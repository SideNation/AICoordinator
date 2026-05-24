# AICO Subagents CLI 기능 추가 플랜

## 개요

`packages/agents` 아래의 단일 agent markdown 파일을 읽어 `claude`, `codex`, `kilo`, `opencode` 플랫폼별 agent 파일로 변환하고, install/update 시 각 플랫폼의 설치 경로에 복사하는 기능을 추가한다.

기존에는 `packages/agents/claude`, `packages/agents/opencode`처럼 플랫폼별 산출물이 이미 존재한다고 가정했지만, 변경 후에는 `packages/agents/<agent>.md`를 단일 소스로 사용한다.

## 핵심 요구사항

- 참고 구현은 `docs/subagents_docs/scripts`의 `agent_lib.py`, `build.py`, `split.py`, `validate.py`를 기준으로 한다.
- 지원 플랫폼은 `claude`, `codex`, `kilo`, `opencode` 4개다.
- 플랫폼은 추후 추가될 수 있으므로 플랫폼별 변환/설치 정보를 registry 구조로 분리한다.
- `codex`는 TOML 파일로 생성한다.
- `install` 명령은 기존 `--target` 옵션을 유지한다.
- `--target` 값은 전체 이름과 약어를 모두 지원한다.
  - `claude`, `cl`
  - `codex`, `co`
  - `kilo`, `ki`
  - `opencode`, `op`
  - `all`
- `--target` 기본값은 `all`이다.
- `--agents`, `-ag` 옵션은 추가하지 않는다.
- `update`는 install 시 설치된 agent 플랫폼 정보를 참조해 동일 플랫폼만 다시 변환/업데이트한다.

## 모델 정책

공통 agent source의 `model` 필드는 아래 3개 tier만 허용한다.

| 공통 model | Claude 변환값 | Codex 변환값 |
|---|---|---|
| `high` | `opus` | `gpt-5.5` |
| `medium` | `sonnet` | `gpt-5.3-codex` |
| `low` | `haiku` | `gpt-5.4-mini` |

정책:

- `model`의 허용값은 `high`, `medium`, `low`뿐이다.
- `midium`은 오타로 간주하고 허용하지 않는다.
- 기존 alias인 `opus`, `sonnet`, `haiku`는 공통 `model` 값으로 허용하지 않는다.
- 플랫폼 native model id를 공통 `model`에 직접 넣는 것도 허용하지 않는다.
- 잘못된 model 값은 install/update 이전 validation 단계에서 에러로 종료한다.

## effort 정책

공통 `effort` 필드를 추가한다.

| 플랫폼 | 변환 필드 |
|---|---|
| `claude` | `effort` |
| `codex` | `reasoningEffort` |
| `kilo` | 플랫폼 문서 기준 pass-through |
| `opencode` | 플랫폼 문서 기준 pass-through |

`effort` 값의 유효 범위는 1차 구현에서 문자열로 유지하되, renderer별로 지원하지 않는 값이 확인되면 platform validation에서 에러 처리한다.

## 단일 agent source 형식

```markdown
---
name: code-reviewer
description: 변경된 코드를 안전성, 가독성, 성능 관점으로 리뷰한다.
model: high
effort: high
tools: [Read, Grep, Glob, Bash]
color: blue
useonly: all

claude:
  permissionMode: default
  maxTurns: 30
  mcpServers: [github]
  memory: project

opencode:
  mode: subagent
  temperature: 0.2
  steps: 40
  permission:
    bash: ask
    edit: deny

codex:
  sandbox_mode: read-only

kilo:
  mode: subagent
  temperature: 0.2
  steps: 40
  permission:
    bash: ask
    edit: deny
  hidden: false
---

당신은 시니어 코드 리뷰어다.
```

`useonly`는 선택 필드이며, 값이 없으면 `all`로 처리한다. 허용값은 `all`, `claude`, `codex`, `kilo`, `opencode`와 각 약어다.

## 구현 설계

### 1. Agent 변환 패키지 추가

새 패키지:

```text
cli/src/agent/
  source.go
  validate.go
  platform.go
  render_claude.go
  render_codex.go
  render_kilo.go
  render_opencode.go
```

역할:

- markdown frontmatter/body 분리
- YAML frontmatter 파싱
- 공통 필드 검증
- `useonly`와 CLI `--target` 선택값을 조합한 최종 플랫폼 결정
- 플랫폼별 frontmatter/TOML 생성
- 변환 warning/error 반환

### 2. Platform registry 추가

`agent.Platform` 구조체를 둔다.

```go
type Platform struct {
    Name       string
    Aliases    []string
    Extension  string
    Render     Renderer
    InstallDir func(scope string) string
}
```

플랫폼 추가 시 registry에 항목만 추가하면 install/update 흐름이 그대로 동작하게 만든다.

### 3. 설치 경로 정의

| 플랫폼 | project scope | user scope |
|---|---|---|
| Claude | `.claude/agents` | `~/.claude/agents` |
| Codex | `.codex/agents` | `~/.codex/agents` |
| Kilo | `.kilo/agents` | `~/.config/kilo/agents` |
| opencode | `.opencode/agents` | `~/.config/opencode/agents` |

Codex 경로와 TOML schema는 구현 전 실제 Codex agent 로딩 규격을 한번 더 확인한다. 확인 결과가 다르면 registry의 Codex 항목만 수정한다.

### 4. install 변경

현재 `install`의 `--target` 중심 agent 설치 구조를 유지하고, 지원 플랫폼을 4개로 확장한다.

기존 사용 예:

```bash
aico install --target claude
aico install --target opencode
aico install --target all
```

변경 후 사용 예:

```bash
aico install
aico install --target all
aico install --target co
aico install --target cl,op
aico install --target codex,kilo,claude,opencode
```

동작:

1. manifest를 로드한다.
2. agent별 source path를 결정한다.
   - `manifest.agents.<name>.source`가 있으면 우선 사용한다.
   - 없으면 `packages/agents/<name>.md`를 사용한다.
3. source를 파싱하고 validate한다.
4. `--target` 선택값과 `useonly`를 교차 적용한다.
5. 대상 플랫폼별로 렌더링한다.
6. 각 플랫폼 install path에 기록한다.
7. `.lock`에 agent 이름, version, 설치 플랫폼 목록을 기록한다.

`--target`은 comma-separated 값을 허용한다. `.lock`에는 입력 alias가 아니라 정규화된 canonical platform name을 안정적인 순서로 저장한다.

### 5. update 변경

`.lock`의 기존 `target` 필드를 유지하고, install 당시 agent 플랫폼 목록을 `target` 문자열에 저장한다.

권장 구조:

```yaml
installs:
  - path: /project/path
    scope: project
    target: claude,codex
    agents:
      code-reviewer: 2026-05-25
```

새 install 기록은 `target: all` 대신 `target: claude,codex,kilo,opencode`처럼 명시적인 canonical list로 저장한다. 이렇게 해야 update 시 실제 설치했던 플랫폼만 정확히 재생성할 수 있다.

update 동작:

1. `.aicorc`의 clone dir에서 `git pull --ff-only`를 시도한다.
2. manifest를 로드한다.
3. `.lock`에서 업데이트 대상 install record를 선택한다.
4. agent version이 변경된 항목만 다시 변환한다.
5. `.lock.target`에 기록된 플랫폼만 다시 쓴다.
6. manifest에서 제거된 agent는 설치된 플랫폼 경로에서만 삭제 여부를 묻는다.

기존 `.lock.target` 값은 그대로 읽는다.

target 해석 규칙:

- `target: claude` -> `[claude]`
- `target: opencode` -> `[opencode]`
- `target: claude,codex` -> `[claude, codex]`
- `target: all` -> legacy record로 간주하고 기존 의미 기준 `[claude, opencode]`로 해석한다.
- 새 install에서 사용자가 `--target all`을 입력하면 `.lock`에는 `claude,codex,kilo,opencode`로 저장한다.

### 6. manifest 구조 확장

현재 manifest entry에 `source`를 읽을 수 있도록 확장한다.

```go
type ManifestEntry struct {
    Source  string `yaml:"source,omitempty"`
    Version string `yaml:"version"`
}
```

예시:

```yaml
agents:
  code-reviewer:
    source: packages/agents/code-reviewer.md
    version: 2026-05-25
```

`source`가 상대 경로이면 clone root 기준으로 해석한다.

### 7. 테스트 계획

단위 테스트:

- `model: high|medium|low`만 통과하는지 확인
- `model: sonnet`, `model: opus`, `model: haiku`, `model: midium`은 실패하는지 확인
- `effort`가 Claude에서는 `effort`, Codex에서는 `reasoningEffort`로 출력되는지 확인
- `useonly`와 `--target` 선택값 교차 적용 확인
- platform override가 공통 필드보다 우선하는지 확인
- Codex TOML 출력이 유효한 TOML인지 확인

통합 테스트:

- temp project에서 `aico install --target cl,co` 실행 후 Claude/Codex 파일 생성 확인
- `aico install` 기본값이 4개 플랫폼 전체 설치인지 확인
- update 시 `.lock.target`에 기록된 플랫폼만 재생성되는지 확인
- manifest version이 동일하면 재설치하지 않는지 확인
- manifest에서 제거된 agent 삭제 프롬프트가 설치 플랫폼에만 적용되는지 확인

### 8. 문서 업데이트

수정 대상:

- `cli/README.md`
- `cli/docs/prd_*.md` 중 최신 문서가 있다면 해당 문서
- command help text

문서에 포함할 내용:

- 단일 source 기반 agent 작성법
- `model` tier 표
- legacy alias 미지원 정책
- `effort` 변환표
- `--target` 사용법
- 플랫폼별 설치 경로
- `.lock.target` 동작

## 구현 순서

1. `cli/src/agent` 패키지 생성
2. 공통 source parser/validator 작성
3. model tier와 effort 변환 구현
4. Claude/opencode renderer 먼저 구현해 기존 기능 대체
5. `install --target`의 플랫폼 값과 alias 확장
6. `.lock.target`의 canonical list 저장/해석 구현
7. `update`를 `.lock.target` 기반으로 수정
8. Kilo renderer 구현
9. Codex TOML schema 확인 후 Codex renderer 구현
10. 단위/통합 테스트 추가
11. README와 help text 업데이트

## 리스크와 확인 필요 사항

- Codex agent TOML 파일의 정확한 schema와 설치 경로는 구현 전에 확인해야 한다.
- Kilo와 opencode의 옵션 호환 범위는 `docs/subagents_docs` 문서를 기준으로 하되, 지원하지 않는 필드는 warning보다 validation error가 더 안전하다.
- 기존 `packages/agents/claude`, `packages/agents/opencode` 산출물 구조를 fallback으로 유지할지 즉시 제거할지 결정이 필요하다.
- 현재 문서에는 `cluade`, `midium`, `cdocs`, `odex` 오타가 있으므로 구현 문서와 CLI help에서는 각각 `claude`, `medium`, `docs`, `codex`로 정규화한다.
