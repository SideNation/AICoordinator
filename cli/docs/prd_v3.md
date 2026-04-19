# PRD v3 — agent splitter CLI (aico)

## 추가 기능
### init 커맨드
- 유저가 자신이 원하는 디렉토리에 aico init 하면 해당 폴더에 있는 .env 환경 파일을 로드한다.
- PACKAGE_GIT_URL 변수가 없다면 git url을 입력 받는다.
- 입력받은 git url을 init을 실행한 폴더에 clone한다.
- packages는 git clone한 폴더 안에 있다.
- users폴더에 .aico 폴더를 만들고 .aicorc 파일을 만들어서 init한 폴더 패스와 clone한 폴더를 저장한다.

### install docs 옵션
- 지금은 agents, skills, docs를 한번에 install하는데 docs는 설치 유무 옵션을 만들어서 default로 설치 하지 않은 것으로 한다.

### install 정보 저장
- .aico폴더에 .lock파일을 만들고 install한 폴더와 packages 버전과 docs 설치 유무를 저장한다.

### update 커맨드
- update 옵션
    - all: .lock 저장된 인스톨한 모든 폴더와 user 폴더에 있는 데이터를 전부 업데이트한다.
        - 폴더가 업다면 해당 정보는 .lock 파일에서 제거한다.
    - user: user 데이터를 업데이트한다.
    - 옵션이 없으면 실행한 폴더에 install 되어 있다면 update한다.
- docs 옵션
    - docs가 설치되어 있지 않다면 docs를 설치한다.
- docs를 설치했는지 안했는지 체크를 해서 설치된 곳은 같이 업데이트 해준다.
#### update 순서
1. .aicorc에 저장된 git 폴더를 pull한다
2. 옵션에 맞게 각 폴더에 데이터를 업데이트한다.