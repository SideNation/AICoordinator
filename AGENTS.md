# AGENTS.md
# 프로젝트 디렉토리 구조

project-root/
├── agents/
├── packages/
│   ├── agents/
│   │   ├── Codex/
│   │   ├── opencode/
│   ├── docs/
│   └── skills/
├── cli/
│   ├── bin/
│   ├── src/
├── codi.yaml
├── lock.yaml
└── README.md

- docs: 일반 문서 저장
    - prd: 제품 개발 계획서 저장(skill, agents등)
- agents: AI에서 생성한 개발중인 서브에이전트 저장 (수동으로 cli를 사용하여 packages/agents에 저장한다.)
- packages: 완성된 파일들 저장
    - agents:
        - Codex: 클로드용 서브 에이전트
        - opencode: 오픈코드용 서브 에이전트
    - docs: 수집한 개발 문서 저장
    - skills: 스킬 저장 (cli 사용)
- cli: 작업한 하나의 에전트용 마크다운 파일을 Codex, opencode용으로 분리하고, agents, skills docs등 버전을 관리하는 앱
    - bin: src에서 빌드된 앱 실행파일 
    - src: go언어로된 소스 파일 폴더
- lock.yaml: docs, agents, skills 버전 관리.

# 스킬 및 agent 생성
생성을 원하는 

## Imported Claude Cowork project instructions
