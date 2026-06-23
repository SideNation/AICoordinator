# AICoordinator (`aico`)

**Claude Code · Codex · Kilo · opencode** 환경에 플러그인(agents·skills·docs·rules·hooks·플랫폼 설정)을 설치·업데이트하는 패키지 관리 CLI.

플러그인은 `packages/plugins/<name>/` 단위로 묶이고, 루트 `manifest.yaml`이 정의·버전의 단일 소스입니다. `aico`는 이를 각 AI 도구가 읽는 경로(`.claude/`, `.codex/`, `.kilo/`, `.opencode/`)에 맞게 설치합니다.

---

## 설치

### 1) 빠른 설치 (권장)

릴리스 바이너리를 받아 `aico`를 PATH에 설치합니다.

**macOS / Linux**
```sh
curl -fsSL https://raw.githubusercontent.com/SideNation/AICoordinator/main/install.sh | sh
```
- `~/.local/bin/aico`에 설치하고, 필요하면 셸 프로필(`.zshrc`/`.bashrc`/`.profile`)에 PATH를 추가합니다.
- OS/아키텍처(`darwin`/`linux`, `arm64`/`amd64`)를 자동 감지합니다.

**Windows (PowerShell)**
```powershell
irm https://raw.githubusercontent.com/SideNation/AICoordinator/main/install.ps1 | iex
```
- `%LOCALAPPDATA%\aico\aico.exe`에 설치하고 사용자 PATH에 등록합니다.

설치 후 확인:
```sh
aico version
```

> 새 셸을 열거나 셸을 재시작해야 PATH 변경이 적용됩니다.

### 2) GitHub Releases에서 수동 다운로드

[Releases](https://github.com/SideNation/AICoordinator/releases/latest)에서 플랫폼에 맞는 파일을 받습니다.

| 플랫폼 | 파일 |
|---|---|
| macOS Apple Silicon | `aico-darwin-arm64` |
| macOS Intel | `aico-darwin-amd64` |
| Linux x86_64 | `aico-linux-amd64` |
| Windows x64 | `aico-windows-amd64.exe` |

**macOS / Linux** — 직접 받기:
```sh
# 예: macOS Apple Silicon
curl -fsSL https://github.com/SideNation/AICoordinator/releases/latest/download/aico-darwin-arm64 -o aico
chmod +x aico
sudo mv aico /usr/local/bin/aico     # 또는 ~/.local/bin (PATH에 있는 곳)
aico version
```
- macOS에서 "확인되지 않은 개발자" 경고가 나오면: `xattr -d com.apple.quarantine $(command -v aico)`

**Windows** — `aico-windows-amd64.exe`를 받아 `aico.exe`로 이름을 바꾸고 PATH에 있는 폴더(예: `%LOCALAPPDATA%\aico`)에 둡니다.

### 3) 소스에서 빌드

Go 1.21+ 필요.
```sh
git clone https://github.com/SideNation/AICoordinator.git
cd AICoordinator/cli
bash build.sh        # bin/ 에 플랫폼별 바이너리 + 현재 플랫폼 native(aico) 생성, ~/.local/bin 에 설치
```
수동 빌드는 [cli/README.md](cli/README.md#빌드) 참고.

---

## 빠른 시작

```sh
# 1) 패키지 저장소 초기화 (manifest.yaml + packages/plugins 가 있는 repo를 clone)
cd ~/work/my-project
PACKAGE_GIT_URL=git@github.com:SideNation/AICoordinator.git aico init

# 2) 설치할 플러그인 확인
aico install           # 인자 없이 실행 → 설치 가능한 플러그인 목록 출력

# 3) 플러그인 설치 (이름 또는 별칭, chain 자동 동반)
aico install csharp
aico install ccgs                 # 별칭 → claude-code-game-studio
aico install --all                # 전체

# 4) 설치 현황 (설치 유무 + 마지막 설치/업데이트 시각)
aico list
aico list -m

# 5) 최신 버전과 동기화 / 삭제
aico update
aico rm csharp
```

자세한 명령·플래그·설치 경로·설정 병합 규칙은 **[cli/README.md](cli/README.md)** 를 보세요.

### 명령 요약

| 명령 | 설명 |
|---|---|
| `aico init` | 패키지 저장소 clone + `~/.aico/.aicorc` 작성 |
| `aico install [plugin...] [--all]` | 플러그인 설치 (chain 동반, `--target`으로 플랫폼 지정) |
| `aico init-tree [plugin...]` | 플러그인의 `_init` 프로젝트 스캐폴드를 워크스페이스에 (재)적용 |
| `aico update` | manifest 버전과 동기화 (변경된 플러그인만 재설치) |
| `aico list` / `ls` | 설치 현황·manifest 비교 (설치 유무 + 시각) |
| `aico rm <plugin>...` | 설치된 플러그인 제거 |
| `aico version` | 버전 출력 |

---

## 저장소 구조

```
AICoordinator/
├── README.md            # (이 파일) 설치 + 빠른 시작
├── install.sh           # macOS/Linux 설치 스크립트
├── install.ps1          # Windows 설치 스크립트
├── manifest.yaml        # 플러그인·docs 정의와 버전 (단일 소스)
├── packages/
│   ├── plugins/<name>/  # 플러그인 (agents/skills/docs/rules/hooks/.claude/.codex/_init)
│   └── docs/            # 수집한 원격 문서
├── docs/                # 일반 문서 (prd 등)
└── cli/                 # aico CLI (Go)
    ├── src/             # 소스
    ├── build.sh         # 멀티 플랫폼 빌드
    └── README.md        # CLI 상세 사용법
```

---

## 라이선스 / 기여

이슈·PR은 [github.com/SideNation/AICoordinator](https://github.com/SideNation/AICoordinator) 에서 받습니다.
