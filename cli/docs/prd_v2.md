# PRD v2 — agent splitter CLI (aico)

## 경로 정의
### opencode: agent
user: ~/.config/opencode/agents/
project: .opencode/agents/

### claude: agent
user: ~/.claude/agents/
project: .claude/agents/

### claude,opencode: skills
user: ~/.claude/skills/
project: .claude/skills/

### claude,opencode: docs
user: ~/.claude/docs/
project: .claude/docs/

### docs

## 추가 기능
- cli에 agents와 skills, docs package들을 프로젝트 또는  user에 복사하는 명령어를 만들어줘  디폴트는 옵션은 project 경로로 복사하고 user 옵션을 하면 user 환경에 복사해줘
- agent같은 경우 opencode, claude, all 옵션을 만들어서 기본 옵션은 claude로 하고 복사
- skills와 docs는 opencode도 claude 폴더에서도 불러 올 수 있으니까 claude 폴더에만 복사