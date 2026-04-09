package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq" // PostgreSQL 드라이버 (인터페이스만 사용하므로 _ 로 임포트)
)

var jwtKey = []byte("sudoku_secret_key_123")
var db *sql.DB

// 클라이언트 요청 구조체 (リクエスト構造体)
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SaveRequest struct {
	Username string      `json:"username"`
	Data     interface{} `json:"data"` // Flutter에서 보낼 JSON 데이터를 그대로 받음
}

// JWT 페이로드 구조체
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// 웹소켓 클라이언트 구조체 (VS 모드용)
type Client struct {
	conn    *websocket.Conn
	matched chan *Client
}

var waitingClient *Client
var mu sync.Mutex

// 데이터베이스 초기화 및 테이블 자동 생성 함수 (DB初期化)
func initDB() {
	var err error
	// NAS의 로컬 IP(192.168.1.163)와 방금 띄운 PostgreSQL 포트(5432)를 사용합니다.
	connStr := "postgres://sudoku:sudoku_secret@192.168.1.163:5433/sudokudb?sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		panic("DB 연결 실패: " + err.Error())
	}

	// 1. 유저 테이블 생성 (ユーザーテーブル作成)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			username VARCHAR(50) PRIMARY KEY,
			password VARCHAR(255) NOT NULL
		);
	`)
	if err != nil {
		panic("users 테이블 생성 실패: " + err.Error())
	}

	// 2. 저장된 게임 테이블 생성 (セーブデータテーブル作成)
	// JSONB 타입으로 스도쿠 판 데이터를 통째로 저장합니다.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS saved_games (
			username VARCHAR(50) PRIMARY KEY REFERENCES users(username),
			game_data JSONB NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		panic("saved_games 테이블 생성 실패: " + err.Error())
	}

	// [테스트용] 프론트엔드 테스트를 위해 기본 계정들을 DB에 미리 넣어둡니다.
	db.Exec(`INSERT INTO users (username, password) VALUES ('whatyousaid', 'password123') ON CONFLICT DO NOTHING;`)
	db.Exec(`INSERT INTO users (username, password) VALUES ('ricky', 'password123') ON CONFLICT DO NOTHING;`)

	fmt.Println("PostgreSQL 데이터베이스 세팅 완료!")
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	// 서버 시작 전 DB 연결
	initDB()

	r := gin.Default()

	// [API 1] 로그인 (ログイン) - 하드코딩 맵 대신 실제 DB를 조회합니다.
	r.POST("/api/login", func(c *gin.Context) {
		var creds Credentials
		if err := c.BindJSON(&creds); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		// DB에서 비밀번호 조회 (DBからパスワードを検索)
		var dbPassword string
		err := db.QueryRow("SELECT password FROM users WHERE username = $1", creds.Username).Scan(&dbPassword)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "존재하지 않는 아이디입니다."})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "DB 에러"})
			}
			return
		}

		if dbPassword != creds.Password {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "비밀번호가 틀렸습니다."})
			return
		}

		expirationTime := time.Now().Add(24 * time.Hour)
		claims := &Claims{
			Username: creds.Username,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expirationTime),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(jwtKey)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "토큰 생성 실패"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": tokenString, "username": creds.Username})
	})

	// [API 2] 게임 저장 (ゲームの保存, セーブ)
	r.POST("/api/save", func(c *gin.Context) {
		var req SaveRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 데이터 형식입니다."})
			return
		}

		// UPSERT 쿼리: 데이터가 없으면 INSERT, 이미 있으면 UPDATE 처리 (UPSERT処理)
		query := `
			INSERT INTO saved_games (username, game_data, updated_at) 
			VALUES ($1, $2, CURRENT_TIMESTAMP)
			ON CONFLICT (username) 
			DO UPDATE SET game_data = EXCLUDED.game_data, updated_at = CURRENT_TIMESTAMP;
		`
		_, err := db.Exec(query, req.Username, req.Data)
		if err != nil {
			fmt.Println("저장 실패:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB 저장 실패"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "성공적으로 저장되었습니다."})
	})

	// [API 3] 저장된 게임 불러오기 (ゲームの読み込み, ロード)
	r.GET("/api/load", func(c *gin.Context) {
		username := c.Query("username")
		if username == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "유저 이름이 필요합니다."})
			return
		}

		var gameData string
		err := db.QueryRow("SELECT game_data FROM saved_games WHERE username = $1", username).Scan(&gameData)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "저장된 게임이 없습니다."})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "DB 조회 실패"})
			}
			return
		}

		// JSON 문자열을 그대로 반환
		c.Data(http.StatusOK, "application/json", []byte(gameData))
	})

	// [API 4] 기존 실시간 대전 웹소켓 (オンライン対戦ソケット)
	r.GET("/ws/match", func(c *gin.Context) {
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		client := &Client{
			conn:    ws,
			matched: make(chan *Client),
		}

		mu.Lock()
		if waitingClient == nil {
			waitingClient = client
			mu.Unlock()

			ws.WriteJSON(gin.H{"type": "waiting", "message": "상대방을 기다리는 중..."})

			disconnect := make(chan struct{})
			go func() {
				_, _, _ = ws.ReadMessage()
				close(disconnect)
			}()

			select {
			case opponent := <-client.matched:
				relayMessages(client.conn, opponent.conn)
			case <-disconnect:
				mu.Lock()
				if waitingClient == client {
					waitingClient = nil
				}
				mu.Unlock()
				ws.Close()
			}
		} else {
			p1 := waitingClient
			waitingClient = nil
			mu.Unlock()

			seed := time.Now().UnixNano()
			matchData := gin.H{"type": "matched", "seed": seed, "difficulty": "medium"}

			p1.conn.WriteJSON(matchData)
			ws.WriteJSON(matchData)

			p1.matched <- client
			relayMessages(ws, p1.conn)
		}
	})

	fmt.Println("서버가 8080 포트에서 실행 중입니다...")
	r.Run(":8080")
}

func relayMessages(src, dst *websocket.Conn) {
	for {
		_, msg, err := src.ReadMessage()
		if err != nil {
			dst.Close()
			src.Close()
			break
		}
		dst.WriteMessage(websocket.TextMessage, msg)
	}
}
