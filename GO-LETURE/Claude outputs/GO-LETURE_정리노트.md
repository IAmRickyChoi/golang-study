# GO-LETURE 학습 정리 노트

> Go + Gin + GORM + PostgreSQL로 만든 **게시판(Board) REST API** 학습 프로젝트
> 모듈명: `go-api` · Go 1.26 · 포트 `:9900`
> ※ 원본 코드는 수정하지 않았고, 읽기만 해서 정리한 문서입니다.

---

## 1. 폴더 구조 한눈에 보기

```
GO-LETURE/
├── go.mod / go.sum            ← 모듈 정의 & 의존성
├── cmd/
│   └── server/
│       └── main.go            ← 진입점: DB 연결 → 의존성 조립 → 라우팅 → 서버 실행
├── internal/                  ← 외부 모듈에서 import 불가 (Go 규칙)
│   ├── model/
│   │   └── board.go           ← Board 구조체 (DB 테이블 ↔ JSON 매핑)
│   ├── repository/
│   │   ├── db.go              ← PostgreSQL 연결 (전역 DB)
│   │   └── board_repository.go← DB 접근 (Create / FindById)
│   ├── service/
│   │   └── board_service.go   ← 요청 처리 (JSON 바인딩 → repo 호출 → 응답)
│   └── handler/               ← (비어 있음)
├── pkg/
│   └── util/                  ← (비어 있음)
└── user/
    └── user.go                ← Profile 구조체 (연습용, 현재 미사용)
```

| 폴더 | 역할 | 비유 (Spring 기준) |
|---|---|---|
| `cmd/server` | 실행 파일 진입점 | `@SpringBootApplication` 메인 클래스 |
| `internal/model` | 데이터 구조 | Entity / DTO |
| `internal/repository` | DB 접근 | Repository / DAO |
| `internal/service` | 비즈니스 로직 (+ 현재는 HTTP 처리까지) | Service (+ Controller) |
| `internal/handler` | HTTP 요청/응답 전용 (예정) | Controller |
| `pkg/util` | 외부 공개 가능한 공통 유틸 | util 패키지 |

---

## 2. 요청 흐름

```
클라이언트
   │  HTTP
   ▼
gin 라우터 (main.go)
   │  r.POST("/board/add") / r.GET("/board/get/:id")
   ▼
BoardService (service)      ← JSON/URI 바인딩, 에러 → 400/500 응답
   │
   ▼
BoardRepository (repository) ← GORM으로 쿼리
   │
   ▼
PostgreSQL (mydb.board 테이블)
```

**의존성 조립 (main.go)** — 생성자 주입 방식

```go
repository.InitDB()                                   // 1. DB 연결
boardRepo := repository.NewBoardRepository(repository.DB) // 2. repo 생성
boardService := service.NewBoardService(boardRepo)    // 3. service에 repo 주입
```

---

## 3. API 목록

| Method | URL | 처리 함수 | 설명 |
|---|---|---|---|
| POST | `/board/add` | `RegisterBoard` | 게시글 등록 (JSON body) |
| GET | `/board/get/:id` | `GetBoardById` | ID로 게시글 1건 조회 |

**등록 요청 예시**
```bash
curl -X POST http://localhost:9900/board/add \
  -H "Content-Type: application/json" \
  -d '{"title":"제목","content":"내용","author":"혁"}'
```

**조회 요청 예시**
```bash
curl http://localhost:9900/board/get/1
```

**응답 규칙**

| 상황 | HTTP | body |
|---|---|---|
| 성공 | 200 | Board JSON + `"code":"200"` |
| JSON/URI 바인딩 실패 | 400 | `{"code":"400","message":...}` |
| DB 에러 (없는 ID 포함) | 500 | `{"code":"500","message":...}` |

---

## 4. 파일별 핵심 포인트

### model/board.go
```go
type Board struct {
    ID        int       `json:"id,omitempty"`
    Title     string    `json:"title,omitempty"`
    Content   string    `json:"content,omitempty"`
    Author    string    `json:"author,omitempty"`
    CreatedAt time.Time `json:"created_at,omitempty"`
    UpdatedAt time.Time `json:"updated_at,omitempty"`
    Code      string    `json:"code,omitempty" gorm:"-"`
}
func (Board) TableName() string { return "board" }
```
- **struct tag**: `json:"..."` → JSON 키 이름, `omitempty` → 빈 값이면 생략
- `gorm:"-"` → DB 컬럼에서 제외 (응답용 필드 `Code`)
- `TableName()` → GORM 기본 복수형(`boards`) 대신 `board` 테이블 사용
- `CreatedAt` / `UpdatedAt` → GORM이 이름 규칙으로 자동 채워줌

### repository/db.go
- `gorm.Open(postgres.Open(dsn), &gorm.Config{})` 로 연결
- 실패 시 `panic` → 서버 시작 자체를 중단
- `db.DB()` 로 내부 `*sql.DB` 를 꺼내 커넥션 풀 설정
- 전역 변수 `var DB *gorm.DB` 에 저장

### repository/board_repository.go
| 메서드 | GORM 코드 | SQL 느낌 |
|---|---|---|
| `Create` | `r.db.Create(board)` | `INSERT INTO board ...` (ID가 board에 채워짐) |
| `FindById` | `r.db.First(&board, id)` | `SELECT * FROM board WHERE id=? ORDER BY id LIMIT 1` |

- 포인터 리시버 `(r *BoardRepository)` 사용
- `.Error` 로 에러만 꺼내서 반환하는 패턴

### service/board_service.go
- `c.ShouldBindJSON(&board)` → body JSON → 구조체
- `c.ShouldBindUri(&req)` → `:id` 경로 파라미터 → 익명 구조체 (`uri:"id"` 태그)
- `gin.H{...}` = `map[string]any` 단축형
- 에러 시 `c.JSON(...)` 후 **반드시 `return`**

### user/user.go
```go
type Profile struct {
    Name string   // 대문자 → 외부 패키지에서 접근 가능 (exported)
    age  int      // 소문자 → 패키지 내부 전용 (unexported)
}
```
- Go의 **공개/비공개 규칙** 연습용으로 보임

---

## 5. 사용 중인 주요 라이브러리

| 라이브러리 | 용도 |
|---|---|
| `github.com/gin-gonic/gin` | HTTP 웹 프레임워크 |
| `gorm.io/gorm` | ORM |
| `gorm.io/driver/postgres` (+ `jackc/pgx/v5`) | PostgreSQL 드라이버 |
| `go-playground/validator` | gin 바인딩 검증 (`binding:"required"`) |
| `go.mongodb.org/mongo-driver/v2` | go.mod에 있으나 코드에서는 미사용 |

---

## 6. 실행 방법

```bash
# PostgreSQL 실행 중 + mydb 데이터베이스 & board 테이블 존재해야 함
go run ./cmd/server
# → "postgres 접속 성공" 출력 후 :9900 에서 대기
```

---

## 7. 코드 읽으면서 발견한 참고 포인트 (수정은 안 함)

다음 공부 단계에서 직접 고쳐보면 좋을 부분들입니다.

1. **`db.go` 커넥션 풀** — `SetMaxIdleConns`가 두 번 호출됨. 두 번째(100)는 아마 `SetMaxOpenConns`를 의도한 것 같아요.
2. **DB 비밀번호가 코드에 하드코딩** — `os.Getenv` 나 `.env` 로 빼는 연습 추천. (Git에 올릴 땐 특히 주의)
3. **없는 ID 조회 시 500** — `errors.Is(err, gorm.ErrRecordNotFound)` 로 구분해서 404를 주면 더 REST스러움.
4. **handler 폴더가 비어 있음** — 지금은 service가 `gin.Context`를 직접 다룸. HTTP 처리를 `handler`로 옮기면 service는 순수 로직만 남아 테스트가 쉬워짐.
5. **go.mod 전부 `// indirect`** — `go mod tidy` 한 번 돌리면 직접 쓰는 gin/gorm이 direct로 정리되고, 안 쓰는 mongo-driver 등은 빠짐.
6. **`user` 패키지** — 현재 어디서도 import 안 됨 (연습 코드).

---

## 8. 한 줄 요약

> **main에서 DB → Repository → Service 순으로 조립하고, Gin 라우터로 `/board` 등록·조회 API 2개를 제공하는 레이어드 구조의 Go 게시판 API.**
