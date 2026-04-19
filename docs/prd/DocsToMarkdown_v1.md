# DocsToMarkdown v1 - PRD

## 1. 개요

웹에 공개된 문서 사이트(예: 프레임워크/라이브러리 공식 문서)의 네비게이션을 자동으로 탐색하여 전체 문서 링크를 수집하고, 각 페이지를 Markdown으로 변환·저장하는 자동화 도구.

- **입력**: 문서 사이트의 진입 URL (네비게이션이 포함된 페이지)
- **출력**: `packages/docs/<site-name>/` 하위의 마크다운 파일 트리 + `index.json` (링크 ↔ 파일 매핑)
- **실행 환경**: Claude Code 내부. Playwright MCP 또는 로컬 Playwright 사용.

## 2. 목표 / 비목표

### 목표
- 토글(접혀 있는 메뉴)을 자동으로 펼쳐 **모든 하위 링크**를 빠짐없이 수집한다.
- 수집한 링크를 순차적으로 읽어 **일관된 Markdown** (프론트매터 + 본문)으로 변환한다.
- `lock.yaml`에 수집 버전/일시를 기록하여 재현성·증분 수집을 보장한다.

### 비목표 (v1)
- 인증이 필요한 문서 사이트 (로그인 플로우)
- JS 렌더링이 극단적으로 복잡한 SPA에서 무한 스크롤 전용 페이지
- 이미지/다이어그램의 재호스팅 (v1에서는 원본 URL 그대로 참조)

## 3. Skill vs Agent — 아키텍처 판단

결론: **Agent 중심 + 보조 Skill** 하이브리드.

| 항목 | Skill 단독 | Agent 단독 | **채택: Agent + Skill** |
|---|---|---|---|
| Playwright 호출 | 어려움 (Skill은 절차 지식) | 자연스러움 | Agent가 브라우저 제어 |
| 다단계 오케스트레이션 (수집→변환→저장) | 단일 턴 한계 | 강점 | Agent의 루프로 처리 |
| 재사용 가능한 변환 규칙 | 강점 | 프롬프트에 매번 포함 필요 | Skill로 고정 |
| 사용자 트리거 | `/docs-to-md` 등 명시적 호출 유리 | Agent는 호출 계약이 느슨 | Skill이 진입점, 내부에서 Agent 사용 |

### 구성 요소
1. **Skill** `docs-to-markdown` — 사용자 진입점. 입력 URL/옵션 파싱, 출력 경로 결정, 하위 Agent 2개 호출, 결과 검증 및 `lock.yaml` 업데이트.
2. **Agent** `link-harvester` — Playwright로 네비게이션 탐색, 토글 unfold, 링크 수집 후 `links.json` 저장.
3. **Agent** `markdown-converter` — `links.json`을 순차 처리. 페이지당 Playwright로 본문 DOM 추출 → Markdown 변환 → 파일 저장.

> 2·3을 별도 Agent로 분리하는 이유: 수집/변환은 실패 지점과 재시도 전략이 다르다. 수집은 "한 번에 끝내야" 일관성이 보장되고, 변환은 "페이지 단위 재시도"가 가능하다. 분리하면 중단 후 재개도 쉬워진다.

## 4. 사용자 플로우

```
사용자: /docs-to-markdown https://example.com/docs --name example
  │
  ▼
[Skill: docs-to-markdown]
  │  옵션 파싱, 출력 디렉토리 결정 (packages/docs/example/)
  │
  ├──▶ [Agent: link-harvester]
  │      1. Playwright 시작, 진입 URL 로드
  │      2. 네비게이션 셀렉터 추론 (nav, aside, [role=navigation] 등)
  │      3. 접혀 있는 toggle 모두 펼침 (반복 스캔까지 상태 고정)
  │      4. 앵커 수집 → 동일 도메인 필터 → 중복 제거
  │      5. links.json 저장 { url, title, depth, parent }
  │
  ├──▶ [Agent: markdown-converter]
  │      1. links.json 로드
  │      2. for each link:
  │         a. Playwright goto, 본문 컨테이너 추출
  │         b. HTML → Markdown 변환 (코드블록/표/링크 보존)
  │         c. frontmatter 생성 { source_url, title, fetched_at }
  │         d. 파일 저장: packages/docs/example/<slug>.md
  │         e. 실패 시 errors.json에 기록 후 계속
  │
  ▼
[Skill: docs-to-markdown]
  │  lock.yaml 업데이트, 요약 리포트 출력
```

## 5. 상세 기능

### 5.1 링크 수집 (link-harvester)
- **토글 펼치기 전략**: `aria-expanded="false"`, `.collapsed`, `.is-closed` 등 일반 패턴 + 사용자 정의 셀렉터(옵션). 클릭 후 DOM 변화가 없을 때까지 반복(최대 N회).
- **스코프 제한**: 진입 URL의 호스트와 경로 prefix 이내만 수집 (외부 링크·Anchor-only 제외).
- **중복 처리**: URL 정규화(쿼리/프래그먼트 제거 여부는 옵션).
- **산출물**: `packages/docs/<name>/links.json`
  ```json
  [{ "url": "...", "title": "...", "depth": 2, "parent": "..." }]
  ```

### 5.2 마크다운 변환 (markdown-converter)
- **본문 추출 우선순위**: `<main>` → `<article>` → `[role=main]` → 사용자 정의 셀렉터 fallback.
- **변환 규칙**:
  - 코드블록: `<pre><code class="language-xxx">` → ``` ```xxx ```
  - 표: GFM 테이블 유지
  - 내부 링크: 수집된 URL이면 상대 경로(.md)로 재작성, 아니면 원본 URL 유지
  - 이미지: `src` 절대화하여 유지 (v1은 재호스팅 안 함)
  - 동영상(YouTube·Vimeo 등): **링크만 추가**. `<iframe>`·`<video>` 임베드는 원본 재생 URL로 환원하여 `[영상: <제목>](<url>)` 형태의 텍스트 링크로 치환 (다운로드/썸네일 임베드 없음)
- **프론트매터**:
  ```yaml
  ---
  source_url: https://...
  title: ...
  fetched_at: 2026-04-19T12:34:56Z
  section: guides/getting-started
  ---
  ```
- **파일 경로 규칙**: URL 경로를 슬러그화 (`/docs/guides/intro` → `guides/intro.md`).

### 5.3 진행 관리 / 재개
- `links.json`에 각 항목의 `status`(pending/done/error) 기록.
- 중단 후 재실행 시 pending/error만 처리.

## 6. 입력 / 출력 계약

### 입력 (Skill 인자)
| 인자 | 타입 | 필수 | 기본값 | 설명 |
|---|---|---|---|---|
| `url` | string | ✅ | - | 문서 진입 URL |
| `--name` | string | ✅ | - | 출력 디렉토리명 (`packages/docs/<name>/`) |
| `--nav-selector` | string | ❌ | 자동 추론 | 네비게이션 루트 셀렉터 |
| `--content-selector` | string | ❌ | 자동 추론 | 본문 셀렉터 |
| `--include` | glob | ❌ | `*` | URL path 필터 |
| `--exclude` | glob | ❌ | - | URL path 제외 |
| `--resume` | bool | ❌ | true | 기존 links.json 이어받기 |

### 출력 구조
```
packages/docs/<name>/
├── links.json          # 수집 결과 + 상태
├── errors.json         # 실패 로그
├── index.md            # 수집한 목차 (트리)
├── <slug1>.md
├── <slug2>.md
└── ...
```

### lock.yaml 반영
```yaml
docs:
  <name>:
    source: https://...
    version: 2026-04-19
    count: 142
    hash: sha256:...
```

## 7. 에러 처리

| 상황 | 동작 |
|---|---|
| Playwright 실행 실패 | Skill에서 진단 메시지 (`npx playwright install` 유도) |
| 네비게이션 미탐지 | `--nav-selector` 수동 지정 요청 |
| 특정 페이지 fetch 실패 | `errors.json`에 기록, 해당 링크 status=error, 전체는 계속 |
| 본문 미탐지 | `--content-selector` 요청. 해당 페이지만 건너뜀 |
| 중복 URL | 정규화 후 1건만 유지 |

## 8. 비기능 요구사항
- **속도**: 페이지당 평균 2초 이내 변환 목표 (순차 처리, rate limit 고려).
- **예의**: `robots.txt` 준수, 요청 간 기본 300ms 지연.
- **결정성**: 같은 사이트·같은 버전이면 같은 슬러그/파일 구조.

## 9. 구현 순서 (v1 일정)

1. **스켈레톤**: Skill 인자 파싱, 출력 디렉토리 생성, 더미 호출로 Agent 계약 확정.
2. **link-harvester**: Playwright navigation + toggle unfold + 링크 수집. 실제 사이트 2~3곳(React 공식, Vue 공식, Tailwind)으로 검증.
3. **markdown-converter**: 본문 추출 + Markdown 변환 + 프론트매터/슬러그.
4. **재개/에러 처리**: status 필드, errors.json, --resume.
5. **lock.yaml 통합**: 수집 버전 기록, CLI 연동.
6. **문서화**: 사용 예시, 트러블슈팅.

## 10. 향후 확장 (v2+)
- 인증 플로우 지원 (쿠키/토큰 주입)
- 이미지 로컬 다운로드 및 상대경로 재작성
- 증분 업데이트 (ETag/Last-Modified 기반 부분 재수집)
- 여러 사이트 병렬 처리
- 변환 품질 평가용 diff 리포트
- 동영상 트랜스크립트 추출 (`--video-mode transcript`, YouTube `youtube-transcript-api`/`yt-dlp`, Vimeo oEmbed). `has_transcripts: true` 프론트매터 마커, `<details>` 블록 포맷 고정.
