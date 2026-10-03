# GO-LETURE 학습 가이드

> 마지막으로 읽은 코드 기준 (board CRUD + comp JWT 토큰 발급)
> 구성: **① 지도 → ② Go 기본 → ③ 파일별 해설 → ④ 문법 총정리 → ⑤ 라이브러리 메서드 사전 → ⑥ 요청 추적 → ⑦ 실수 모음 → ⑧ 퀴즈**

---

## ① 프로젝트 지도

```
GO-LETURE/
├── go.mod                       모듈 이름(go-api) + 의존성 목록
├── go.sum                       의존성 체크섬 (자동 생성, 직접 수정 X)
├── .env                         비밀 설정값 (DB 접속 정보)
├── cmd/
│   └── server/main.go           ★ 시작점
├── internal/
│   ├── config/config.go         .env → Config 구조체
│   ├── model/
│   │   ├── board.go             게시글 구조체
│   │   └── comp.go              업체 구조체
│   ├── repository/
│   │   ├── db.go                DB 연결
│   │   ├── board_repository.go  게시글 DB 작업
│   │   └── comp_repository.go   업체 DB 작업
│   ├── service/
│   │   ├── board_service.go     게시글 API 처리
│   │   └── auth_service.go      JWT 발급
│   └── handler/                 (비어 있음)
├── pkg/util/                    (비어 있음)
└── user/user.go                 연습용 (미사용)
```

### 추천 읽는 순서
`main.go` → `config.go` → `db.go` → `model/*` → `repository/*` → `service/*`
(조립하는 곳을 먼저 보고, 조립되는 부품을 하나씩 내려가며 읽기)

### 레이어 역할 (Java/Spring 비교)

| Go 폴더 | 역할 | Spring 대응 |
|---|---|---|
| `cmd/server` | main 함수, 조립 | `@SpringBootApplication` |
| `config` | 설정 로드 | `application.yml` + `@ConfigurationProperties` |
| `model` | 데이터 구조 | `@Entity` / DTO |
| `repository` | DB 접근 | `JpaRepository` |
| `service` | 로직 (+ 현재 HTTP 처리까지) | `@Service` (+ `@RestController`) |

---

## ② Go 기본 개념

### 2-1. module / package / import

```go
// go.mod
module go-api          // ← 이 프로젝트의 "이름". import 경로의 시작점
go 1.26.0
```

```go
package service        // ← 파일 첫 줄: 이 파일이 속한 패키지 (= 폴더 이름과 맞추는 게 관례)

import (
    "go-api/internal/model"         // 내 프로젝트 패키지: 모듈명/폴더경로
    "time"                          // 표준 라이브러리
    "github.com/gin-gonic/gin"      // 외부 라이브러리
)
```

- **같은 폴더 = 같은 패키지.** `db.go`와 `board_repository.go`는 둘 다 `package repository`라서 서로의 변수/함수를 import 없이 바로 씀.
- 사용할 때는 `패키지명.이름` → `model.Board`, `repository.DB`, `gin.Default()`
- import 했는데 안 쓰면 **컴파일 에러** (Go는 엄격함)

### 2-2. 특수 폴더 이름의 의미

| 폴더 | 의미 |
|---|---|
| `cmd/` | 실행 파일(main 패키지)을 두는 관례 |
| `internal/` | **Go 컴파일러가 강제**: 이 모듈 밖에서는 import 불가 |
| `pkg/` | 외부에 공개해도 되는 코드 관례 (강제 아님) |

### 2-3. 대문자 = public, 소문자 = private

```go
type Profile struct {
    Name string   // 대문자 → 다른 패키지에서 접근 가능 (exported)
    age  int      // 소문자 → 같은 패키지 안에서만
}
```
함수, 변수, 구조체, 필드 전부 같은 규칙. Java의 `public/private` 키워드 대신 **첫 글자**로 결정.
→ `repository.DB`, `config.Load()`가 대문자인 이유. `getEnv()`, `jwtKey`는 소문자라 패키지 내부 전용.

### 2-4. go.mod / go.sum 명령어

| 명령 | 하는 일 |
|---|---|
| `go get github.com/joho/godotenv` | 라이브러리 추가 |
| `go mod tidy` | 안 쓰는 의존성 제거 + 필요한 것 추가 + `// indirect` 정리 |
| `go run ./cmd/server` | 컴파일 + 실행 |
| `go build ./...` | 전체 컴파일만 (에러 확인용) |

`// indirect` = "내 코드가 직접 import하지 않고 다른 라이브러리가 필요로 하는 것". `go mod tidy` 하면 gin, gorm 등은 direct로 바뀜.

---

## ③ 파일별 해설

### 3-1. `cmd/server/main.go`

```go
package main                    // 실행 파일은 반드시 package main + func main()

func main() {
    cfg := config.Load()                         // ① .env 읽기
    repository.InitDB(cfg.DSN())                 // ② DB 연결 → 전역 repository.DB에 저장

    boardRepo := repository.NewBoardRepository(repository.DB)   // ③ 부품 생성
    boardService := service.NewBoardService(boardRepo)          //    service에 repo 주입

    compRepo := repository.NewCompRepository(repository.DB)
    authService := service.NewAuthService(compRepo)

    r := gin.Default()                           // ④ 라우터 생성

    r.POST("/board/add", boardService.RegisterBoard)    // ⑤ URL ↔ 함수 연결
    r.GET("/board/get/:id", boardService.GetBoardById)
    r.PUT("/board/update", boardService.UpdateBoard)
    r.DELETE("/board/delete/:id", boardService.DeleteBoard)
    r.POST("/auth/token", authService.MakeToken)

    r.Run(":9900")                               // ⑥ 서버 시작 (여기서 계속 대기)
}
```

**포인트**
- `boardService.RegisterBoard` ← 괄호 없음! 함수를 **실행하지 않고 값으로 넘김** (method value). Gin이 요청 올 때 대신 호출해 줌.
- **의존성 주입(DI)**: Spring은 `@Autowired`로 자동, Go는 `main`에서 **손으로** 조립. 흐름이 눈에 다 보이는 게 장점.
- `:id` = 경로 파라미터. 있으면 URL에 값이 **반드시** 있어야 매칭됨.

---

### 3-2. `internal/config/config.go`

```go
type Config struct {
    ServerPort string
    DBHost     string
    ...
}

func Load() *Config {
    if err := godotenv.Load(); err != nil {          // .env 내용을 OS 환경변수로 등록
        log.Println(".env 파일 없음 → 시스템 환경변수 사용")
    }
    return &Config{                                   // 구조체를 만들어 "주소"를 반환
        ServerPort: getEnv("SERVER_PORT", "9900"),
        DBUser:     mustGetEnv("DB_USER"),
        ...
    }
}

func (c *Config) DSN() string {                       // Config에 붙은 메서드
    return fmt.Sprintf("host=%s user=%s ...", c.DBHost, c.DBUser, ...)
}

func getEnv(key, defaultValue string) string {        // 같은 타입 매개변수는 묶어 쓰기 가능
    if v := os.Getenv(key); v != "" {
        return v
    }
    return defaultValue
}
```

**포인트**
- `&Config{...}` → 구조체 생성 + 그 주소(포인터) 반환. `*Config`가 반환 타입.
- `getEnv(key, defaultValue string)` = `getEnv(key string, defaultValue string)`의 축약
- `if v := ...; 조건 {}` → **if 안에서만 쓰는 변수** 선언 (2-7 참고)
- `log.Fatalf` = 로그 출력 + **프로그램 즉시 종료**

---

### 3-3. `internal/repository/db.go`

```go
var DB *gorm.DB                         // 패키지 전역 변수 (대문자 → 외부 접근 가능)

func InitDB(dsn string) {
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        panic("postgres 연결 실패: " + err.Error())
    }
    fmt.Println("postgres 접속 성공")

    postgresDB, _ := db.DB()            // _ = 두 번째 반환값(error) 버리기
    postgresDB.SetMaxIdleConns(10)
    postgresDB.SetMaxOpenConns(100)
    postgresDB.SetConnMaxLifetime(time.Hour)
    DB = db                             // := 아니라 = (이미 선언된 전역 변수에 대입)
}
```

**포인트**
- `gorm.Open(드라이버, 설정)` → 드라이버만 바꾸면 MySQL, SQLite도 같은 방식
- `db.DB()` → GORM 안의 표준 `*sql.DB`를 꺼냄 (커넥션 풀은 이쪽에서 설정)
- **`DB = db` vs `DB := db`** ← 중요! `:=`를 쓰면 함수 안에 **새 지역 변수**가 생겨서 전역 DB는 계속 nil → 나중에 nil pointer 에러
- `panic` vs `log.Fatal`: 둘 다 프로그램 종료. panic은 스택 트레이스 출력 + `recover`로 잡을 수 있음

---

### 3-4. `internal/model/board.go`, `comp.go`

```go
type Board struct {
    ID        int       `json:"id,omitempty"`
    Title     string    `json:"title,omitempty"`
    CreatedAt time.Time `json:"created_at,omitempty"`
    UpdatedAt time.Time `json:"updated_at,omitempty"`
    Code      string    `json:"code,omitempty" gorm:"-"`
}
func (Board) TableName() string { return "board" }

type Comp struct {
    ID    string `json:"id,omitempty" gorm:"primaryKey"`
    Name  string `json:"name,omitempty"`
    Token string `json:"token,omitempty"`
    Code  string `json:"code,omitempty" gorm:"-"`
}
func (c *Comp) TableName() string { return "comp" }
```

**struct tag** (백틱 `` ` `` 안의 메타데이터) — Java 어노테이션 비슷

| 태그 | 의미 |
|---|---|
| `json:"title"` | JSON 키 이름을 `title`로 |
| `json:",omitempty"` | 값이 비어 있으면(0, "", nil) JSON에서 생략 |
| `json:"-"` | JSON에서 완전히 제외 |
| `gorm:"primaryKey"` | 기본키 지정 |
| `gorm:"-"` | DB 컬럼에서 제외 (응답 전용 필드) |

**GORM 이름 규칙 (Convention)**
- 필드 `ID` → 자동으로 기본키. `Board`는 태그 없어도 됨. `Comp`는 string이라 명시함.
- `CreatedAt`, `UpdatedAt` → 생성/수정 시 **자동으로 현재 시간** 채움
- 필드 `CreatedAt` → 컬럼 `created_at` (snake_case 자동 변환)
- 테이블명 기본값은 **복수형 snake_case** (`Board` → `boards`) → `TableName()`으로 덮어씀

**`TableName()`의 정체**: GORM이 정해 둔 `Tabler` 인터페이스 `TableName() string`을 구현한 것. Go는 `implements` 키워드 없이 **메서드만 맞으면 자동으로 인터페이스 구현** (duck typing).

**`(Board)` vs `(c *Comp)`**: 리시버 차이. `(Board)`는 값 리시버(변수 이름 생략), `(c *Comp)`는 포인터 리시버. 둘 다 동작하지만 한 프로젝트 안에서는 통일하는 게 좋음.

---

### 3-5. `internal/repository/board_repository.go`, `comp_repository.go`

```go
type BoardRepository struct {
    db *gorm.DB                      // 소문자 → 외부에서 직접 못 만짐 (캡슐화)
}

func NewBoardRepository(db *gorm.DB) *BoardRepository {   // 생성자 패턴
    return &BoardRepository{db: db}
}

func (r *BoardRepository) Create(board *model.Board) error {
    return r.db.Create(board).Error
}

func (r *BoardRepository) FindById(id int) (*model.Board, error) {   // 반환값 2개
    var board model.Board
    err := r.db.First(&board, id).Error
    return &board, err
}

func (r *BoardRepository) Update(board *model.Board) error {
    return r.db.Save(board).Error
}

func (r *BoardRepository) Delete(id int) error {
    return r.db.Delete(&model.Board{}, id).Error
}
```

```go
// comp_repository.go
func (r *CompRepository) FindById(id string) (*model.Comp, error) {
    var comp model.Comp
    err := r.db.First(&comp, "id = ? ", id).Error    // ← string 키는 조건문으로!
    return &comp, err
}
```

**포인트**
- **생성자 패턴**: Go에는 생성자 문법이 없음 → `NewXxx()` 함수를 만드는 게 관례
- **`(r *BoardRepository)`** = 메서드 리시버. Java의 `this`를 `r`이라는 이름으로 직접 씀
- **GORM 체이닝**: `r.db.Create(...)`는 `*gorm.DB`를 반환 → `.Error`로 에러만 꺼냄
- `First(&board, id)` ← **`&` 필수**. 결과를 채워 넣을 "주소"를 줘야 함
- **int 키 vs string 키**: `First(&b, 5)`는 OK. string은 `First(&c, "id = ?", id)`로 써야 함 (그냥 문자열 넣으면 SQL 조건문으로 해석됨 → SQL 인젝션 위험)
- `?` = 플레이스홀더. 값이 안전하게 바인딩됨 (Java `PreparedStatement`와 같음)

**GORM 메서드 → SQL**

| GORM | SQL |
|---|---|
| `Create(&b)` | `INSERT INTO board (...) VALUES (...)` → 생성된 ID가 b.ID에 채워짐 |
| `First(&b, 5)` | `SELECT * FROM board WHERE id=5 ORDER BY id LIMIT 1` |
| `Save(&b)` | 기본키 있으면 `UPDATE` (**모든 컬럼**), 없으면 `INSERT` |
| `Delete(&Board{}, 5)` | `DELETE FROM board WHERE id=5` |

---

### 3-6. `internal/service/board_service.go`

```go
func (s *BoardService) RegisterBoard(c *gin.Context) {
    var board model.Board

    if err := c.ShouldBindJSON(&board); err != nil {     // body JSON → 구조체
        c.JSON(400, gin.H{"code": "400", "message": err.Error()})
        return                                            // ★ 꼭 return
    }

    if err := s.repo.Create(&board); err != nil {
        c.JSON(500, gin.H{"code": "500", "message": err.Error()})
        return
    }

    board.Code = "200"
    c.JSON(200, board)                                    // 구조체 → JSON 응답
}

func (s *BoardService) GetBoardById(c *gin.Context) {
    var req struct {                                      // 익명 구조체 (이 함수에서만 사용)
        Id int `uri:"id" binding:"required"`
    }
    if err := c.ShouldBindUri(&req); err != nil { ... }   // URL :id → req.Id
    board, err := s.repo.FindById(req.Id)
    ...
}
```

**포인트**
- **`c *gin.Context`** = 요청 + 응답을 다 들고 있는 객체 (Java의 `HttpServletRequest` + `Response`)
- **`ShouldBindJSON`** = body 읽기 / **`ShouldBindUri`** = URL 경로 파라미터 읽기
- **`uri:"id"`** = 라우트의 `:id`와 이름 연결. **`binding:"required"`** = 값 없으면 에러
- **`gin.H`** = `map[string]any`의 별칭. 간단한 JSON 응답 만들 때 사용
- `c.JSON()`은 응답을 **쓰기만** 하고 함수를 끝내지 않음 → `return` 필수

---

### 3-7. `internal/service/auth_service.go` (JWT)

```go
var jwtKey = []byte("my_secret_key_1234")      // string → []byte 변환

func (s *AuthService) GenerateToken(id string) (string, error) {
    claims := jwt.MapClaims{                    // 토큰에 담을 정보 (map)
        "id":  id,
        "exp": time.Now().Add(time.Hour * 24).Unix(),   // 만료: 지금+24시간 (초 단위 숫자)
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)   // 토큰 객체 생성
    return token.SignedString(jwtKey)           // 비밀키로 서명 → 문자열 완성
}

func (s *AuthService) MakeToken(c *gin.Context) {
    var param model.Comp
    // 1. body 바인딩 → 2. ID 빈값 체크 → 3. DB 조회
    // 4. 토큰 생성 → 5. comp.Token에 넣고 Save → 6. 응답
}
```

**JWT 구조**

```
eyJhbGciOiJIUzI1NiJ9 . eyJpZCI6InRlc3QiLCJleHAiOjE3...} . SflKxwRJSMeKKF2QT4fw...
└─── Header ───────┘   └─── Payload (claims) ─────────┘   └─── Signature ──────┘
  알고리즘(HS256)          id, exp                          비밀키로 만든 서명
```
- Header와 Payload는 **Base64 인코딩일 뿐 암호화 아님** → 누구나 디코딩해서 볼 수 있음. 비밀번호 같은 건 절대 넣지 말 것
- Signature 덕분에 **내용을 조작하면 검증에서 걸림**
- `exp`는 표준 claim 이름. 나중에 토큰 검증(`jwt.Parse`) 시 자동으로 만료 체크됨

**HS256 vs ES256**

| | HS256 | ES256 |
|---|---|---|
| 키 | 비밀 문자열 1개 `[]byte` | 개인키 + 공개키 쌍 |
| 서명/검증 | 같은 키 | 개인키로 서명, 공개키로 검증 |
| 용도 | 서버 하나가 발급·검증 | 발급 서버와 검증 서버가 다를 때 |

---

## ④ 문법 총정리

### 4-1. 변수 선언

```go
var DB *gorm.DB              // 타입만 선언 (zero value: nil)
var board model.Board        // 구조체 zero value: 모든 필드 0 / "" 
var jwtKey = []byte("...")   // 타입 추론
cfg := config.Load()         // 함수 안에서만 쓰는 단축 선언 (선언 + 대입)
DB = db                      // 이미 있는 변수에 대입
```

| | `:=` | `=` |
|---|---|---|
| 의미 | 새 변수 만들고 대입 | 기존 변수에 대입 |
| 위치 | 함수 안에서만 | 어디서나 |

**zero value**: Go는 초기화 안 하면 자동으로 `int → 0`, `string → ""`, `bool → false`, 포인터 → `nil`.
→ 그래서 body에 `"id"`가 없으면 `board.ID == 0`이 됐던 것.

### 4-2. 다중 반환 + 에러 처리 (Go의 핵심 패턴)

```go
board, err := s.repo.FindById(5)
if err != nil {
    // 에러 처리
    return
}
// 정상 흐름
```
- Go에는 **try-catch가 없음**. 에러를 **반환값**으로 돌려주고 호출한 쪽에서 바로 확인
- 함수 시그니처: `func FindById(id int) (*model.Board, error)` ← 반환 타입을 괄호로 묶음
- 쓰기 싫은 반환값은 `_`로 버림: `postgresDB, _ := db.DB()`

### 4-3. if 문 안의 짧은 선언

```go
if err := c.ShouldBindJSON(&board); err != nil {
    ...
}
// 여기서는 err 사용 불가 (스코프가 if 안으로 한정)
```
= 아래와 같음
```go
err := c.ShouldBindJSON(&board)
if err != nil { ... }
```

### 4-4. 포인터 `&` 와 `*`

```go
var board model.Board     // 실제 값
p := &board               // & = "주소를 줘"     → 타입 *model.Board
p.Title = "a"             // Go는 포인터여도 . 으로 바로 필드 접근 (자동 역참조)
```

| 기호 | 위치 | 의미 |
|---|---|---|
| `*T` | 타입 자리 | "T를 가리키는 포인터 타입" |
| `&x` | 값 자리 | "x의 주소" |

**왜 `&`를 넘기나?** Go는 기본이 **값 복사**. `ShouldBindJSON(board)`처럼 넘기면 복사본을 채우고 버려짐 → 원본은 그대로. `&board`를 넘겨야 원본이 채워짐.

### 4-5. 구조체 & 메서드

```go
type BoardService struct {           // 클래스의 "필드" 부분
    repo *repository.BoardRepository
}

func (s *BoardService) RegisterBoard(c *gin.Context) { ... }   // 클래스의 "메서드" 부분
//   └─ 리시버: 이 메서드가 어느 타입에 붙는지
```
- Go에는 `class`가 없음. **struct + 메서드**로 같은 역할
- **포인터 리시버 `(s *T)`** 를 쓰는 이유: 복사 비용 없음 + 필드 수정 가능 → 대부분 이걸 씀

### 4-6. 익명 구조체

```go
var req struct {
    Id int `uri:"id" binding:"required"`
}
```
이름 없이 한 번만 쓰는 구조체. 요청 파라미터 받을 때 자주 씀.

### 4-7. map & 복합 리터럴

```go
gin.H{"code": "400", "message": "..."}     // map[string]any
jwt.MapClaims{"id": id, "exp": 123}        // map[string]interface{}
&Config{ServerPort: "9900"}                // 구조체 리터럴 + 주소
```

### 4-8. 타입 변환

```go
[]byte("my_secret")          // string → byte 슬라이스
err.Error()                  // error → string
"연결 실패: " + err.Error()   // string 연결은 +
```

### 4-9. 인터페이스 (암묵적 구현)

```go
type error interface { Error() string }        // Go 내장 인터페이스
type Tabler interface { TableName() string }  // GORM 인터페이스
```
메서드 시그니처만 맞으면 **자동으로 구현한 것으로 인정**. `implements` 선언 없음.

---

## ⑤ 라이브러리 메서드 사전

### Gin (`github.com/gin-gonic/gin`)

| 메서드 | 설명 |
|---|---|
| `gin.Default()` | 로거 + 패닉 복구 미들웨어가 붙은 라우터 생성 |
| `r.GET/POST/PUT/DELETE(path, handler)` | 라우트 등록 |
| `r.Run(":9900")` | 서버 시작 (블로킹) |
| `c.ShouldBindJSON(&x)` | body JSON → 구조체 |
| `c.ShouldBindUri(&x)` | `:id` 경로 파라미터 → 구조체 (`uri` 태그) |
| `c.Param("id")` | `:id` 값을 문자열로 바로 꺼내기 (Bind 대신 간단히) |
| `c.Query("page")` | `?page=1` 쿼리스트링 읽기 |
| `c.JSON(code, obj)` | JSON 응답 |
| `gin.H{}` | `map[string]any` 단축 |

> `ShouldBind...` vs `Bind...`: Should는 에러를 **반환만**, Bind는 실패 시 **자동으로 400 응답**까지 함. 보통 Should를 쓰고 직접 처리.

### GORM (`gorm.io/gorm`)

| 메서드 | 설명 |
|---|---|
| `gorm.Open(dialector, &gorm.Config{})` | DB 연결 |
| `db.DB()` | 내부 `*sql.DB` 꺼내기 |
| `db.Create(&x)` | INSERT |
| `db.First(&x, id)` | PK로 1건 조회 (없으면 `gorm.ErrRecordNotFound`) |
| `db.First(&x, "col = ?", v)` | 조건으로 1건 조회 |
| `db.Find(&list)` | 전체 조회 (슬라이스에 담음) |
| `db.Where("a = ?", v).Find(&list)` | 조건 조회 |
| `db.Save(&x)` | 전체 컬럼 UPDATE (PK 없으면 INSERT) |
| `db.Model(&x).Updates(obj)` | **0이 아닌 필드만** UPDATE |
| `db.Delete(&T{}, id)` | DELETE |
| `.Error` | 실행 결과 에러 |

### database/sql (`*sql.DB`)

| 메서드 | 설명 |
|---|---|
| `SetMaxIdleConns(n)` | 대기 연결 최대 수 |
| `SetMaxOpenConns(n)` | 동시 연결 최대 수 |
| `SetConnMaxLifetime(d)` | 연결 하나의 수명 |

### 표준 라이브러리

| 함수 | 설명 |
|---|---|
| `fmt.Println(a)` | 출력 + 줄바꿈 |
| `fmt.Sprintf("%s %d", s, n)` | 포맷 문자열 **반환** (출력 X). `%s` 문자열, `%d` 정수, `%v` 아무거나 |
| `os.Getenv("KEY")` | 환경변수 읽기 (없으면 "") |
| `log.Println(a)` | 시간 붙여 로그 출력 |
| `log.Fatalf(fmt, ...)` | 로그 출력 후 종료 |
| `time.Now()` | 현재 시간 |
| `t.Add(time.Hour * 24)` | 시간 더하기 |
| `t.Unix()` | 1970년부터의 초 (int64) |
| `time.Hour` | 1시간을 나타내는 `time.Duration` 상수 |

### godotenv / jwt

| 함수 | 설명 |
|---|---|
| `godotenv.Load()` | 현재 폴더의 `.env`를 환경변수로 등록 |
| `jwt.MapClaims{}` | 토큰 payload (map) |
| `jwt.NewWithClaims(method, claims)` | 서명 전 토큰 객체 |
| `jwt.SigningMethodHS256` | 서명 알고리즘 |
| `token.SignedString(key)` | 서명해서 최종 문자열 생성 |

---

## ⑥ 요청 하나 끝까지 따라가기: `POST /auth/token`

```
Postman: POST /auth/token  body {"id":"test"}
  │
  ▼ [Gin 라우터] Method=POST, URL=/auth/token 일치 → authService.MakeToken 호출
  │
  ▼ [MakeToken]
  │   ShouldBindJSON(&param)          → param.ID = "test"
  │   param.ID == "" ?                → 아니니까 통과
  │   s.repo.FindById("test")
  │       │
  │       ▼ [CompRepository.FindById]
  │           SELECT * FROM comp WHERE id = 'test' LIMIT 1
  │           → comp = {ID:"test", Name:"테스트 기업", Token:""}
  │
  │   s.GenerateToken("test")
  │       claims = {id:"test", exp:1790...}
  │       HS256 + jwtKey 로 서명 → "eyJhbGci..."
  │
  │   comp.Token = "eyJhbGci..."
  │   s.repo.Update(comp)             → UPDATE comp SET name=..., token=... WHERE id='test'
  │
  ▼   c.JSON(200, {"code":"200","data":comp})
Postman: 200 OK + 토큰
```

---

## ⑦ 이 프로젝트에서 실제로 나온 실수 모음

| 증상 | 원인 | 해결 |
|---|---|---|
| `404 page not found` | Method나 URL이 라우트와 다름 (`:id` 누락, POST↔PUT) | 라우트 등록과 요청을 1:1 대조 |
| body에 id 넣었는데도 404 | 라우터는 body를 안 봄 | URL 자체를 맞춰야 함 |
| 응답 JSON이 두 개 `}{` | 에러 응답 뒤 `return` 누락 | `c.JSON` 후 `return` |
| `ECDSA sign expects *ecdsa.PrivateKey` | 문자열 키에 ES256 사용 | `SigningMethodHS256` |
| Update 시 "not found" | id를 안 받아서 zero value 0 | 바인딩 위치(body/uri) 확인 |
| `SetMaxIdleConns` 두 번 | 오타 | 두 번째는 `SetMaxOpenConns` |
| 코드 수정했는데 반영 안 됨 | 서버 재시작 안 함 | Ctrl+C → `go run` 다시 |

---

## ⑧ 확인 퀴즈

1. `internal` 폴더에 넣은 패키지는 어떤 제약이 있나?
2. `cfg := config.Load()`와 `DB = db`에서 `:=`와 `=`의 차이는?
3. `c.ShouldBindJSON(board)`처럼 `&` 없이 넘기면 어떻게 되나?
4. `gorm:"-"`와 `json:"-"`의 차이는?
5. `First(&comp, id)` 대신 `First(&comp, "id = ?", id)`를 쓴 이유는?
6. `Save`와 `Updates`의 차이는?
7. JWT payload에 비밀번호를 넣으면 안 되는 이유는?
8. `r.POST("/board/add", boardService.RegisterBoard)`에서 `RegisterBoard` 뒤에 `()`가 없는 이유는?
9. Go에서 try-catch 대신 쓰는 에러 처리 방식은?
10. `getEnv`는 소문자, `Load`는 대문자인 이유는?

<details>
<summary>정답 보기</summary>

1. 같은 모듈(go-api) 밖에서는 import할 수 없다. 컴파일러가 강제한다.
2. `:=`는 새 변수 선언+대입(함수 안 전용), `=`는 이미 있는 변수에 대입. 전역 `DB`에 `:=`를 쓰면 지역 변수가 새로 생겨 전역은 nil로 남는다.
3. 복사본이 채워지고 버려져서 원본 `board`는 비어 있다. (실제로는 Gin이 포인터가 아니라고 에러를 낸다)
4. `gorm:"-"`는 DB 컬럼에서 제외, `json:"-"`는 JSON 입출력에서 제외.
5. ID가 string이라서. 문자열을 그냥 넘기면 GORM이 SQL 조건문으로 해석해 의도와 다르게 동작하고 인젝션 위험도 있다.
6. `Save`는 모든 컬럼을 덮어씀(빈 값도), `Updates`는 0/빈 값이 아닌 필드만 갱신.
7. Payload는 Base64 인코딩일 뿐 암호화가 아니라 누구나 디코딩해서 읽을 수 있다.
8. 지금 실행하는 게 아니라 함수 자체를 넘겨 두고, 요청이 왔을 때 Gin이 호출하게 하려고.
9. 에러를 반환값으로 받아서 `if err != nil`로 바로 확인.
10. 소문자는 패키지 내부 전용, 대문자는 외부 공개. `Load`는 main에서 호출해야 하니까 대문자.
</details>

---

## 다음 단계 추천

1. `auth_service.go` 수정 (HS256 + `return` 2개)
2. `jwtKey` → `.env`의 `JWT_SECRET`으로 이동
3. **토큰 검증 미들웨어** 만들기: 요청 헤더 `Authorization: Bearer <토큰>`을 `jwt.Parse`로 검증 → 통과한 요청만 `/board/*` 접근 허용 (Gin `r.Group` + `Use`)
4. `handler` 폴더로 HTTP 처리 분리 → service는 순수 로직만
5. `errors.Is(err, gorm.ErrRecordNotFound)`로 404 구분
