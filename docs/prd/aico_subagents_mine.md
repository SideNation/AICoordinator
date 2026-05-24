# cli 기능 추가
## 개요
스킬을 각 플랫폼에 맞게 재생성하고 각자 폴더에 복사하는 기능

## 작업 개요
- docs/subagents_docs/scripts 스크립트를 참고 해서 만든다
  - 이제 packages/agents 폴더에 opencode, claude 구분이 없어졌다.
- 플랫폼은 kilo, codex, cluade, opencode 총 4개 (추후에 추가가 될 수 도 있음)
  - codex는 toml 파일을 사용한다.
- model은 codex,claude이외 나중에 다른 모델 추가가 필요하기 때문에 기본적으로 3가지로 나누고 컨버팅시에 각 플랫폼에 맞게 변경한다.
  - high: claude=opus, codex=gpt-5.5
  - midium: claude=sonnet, codex=gpt-5.3-codex
  - low: claude=haiku, codex=gpt-5.4-mini
- effort: 옵션을 추가한다. 
  - claude=effort, codex=reasoningEffort
- kilo와 opencode는 cdocs/subagents_docs에있는 문서들을 참고해서 effort외에 odex와, claude의 옵션들을 모두 포함시킨다
- install시에 옵션으로 --agents/-ag가 있다 옵션 값을
  -  codex/co, kilo/ki,claude/cl,opencode/op 전체 단어와 약어로 둘다 사용 가능하게 만든다
  - default는 all이다
- update 일때도 install 각 플랫폼의 설치 여부를 참조해서 packages/agents폴더에 있는 데이터를 각 플랫폼에 맞게 update를 해줘야함

## 예시

### 예시 A — 최소 통합 파일 (claude/opencode만 사용)

```markdown
---
name: doc-fetcher
description: 외부 문서 사이트의 페이지를 끌어와 markdown으로 변환한다.
model: haiku
tools: [Read, Write, Bash, Grep]
color: blue

claude:
  permissionMode: auto
  maxTurns: 40

opencode:
  mode: subagent
  temperature: 0.1
  steps: 50

useonly: both
---

당신은 문서 변환 전문가다. ...
```

### 예시 B — 4 플랫폼 전체 활용

```markdown
---
name: code-reviewer
description: 변경된 코드를 안전성·가독성·성능 관점으로 리뷰한다.
model: sonnet
effort: high
tools: [Read, Grep, Glob, Bash]
color: '#3b82f6'

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
  temperature: 0.2
  steps: 40
  permission:
    bash: {allow: ['git diff *', 'git log *', 'ls *', 'cat *']}
    edit: deny
    task: deny
  hidden: false
---

당신은 시니어 코드 리뷰어다. ...
```

### 예시 C — 일부 플랫폼만 빌드

```markdown
---
name: claude-only-helper
description: Claude Code 전용 보조 에이전트.
model: opus
useonly: claude

claude:
  permissionMode: bypassPermissions
  hooks:
    PreToolUse: ...
---
...
```
