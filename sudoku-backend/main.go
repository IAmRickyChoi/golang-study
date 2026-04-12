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
	_ "github.com/lib/pq"
)

var jwtKey = []byte("sudoku_secret_key_123")
var db *sql.DB

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SaveRequest struct {
	Username string      `json:"username"`
	Data     interface{} `json:"data"`
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type Client struct {
	conn    *websocket.Conn
	matched chan *Client
}

var waitingClient *Client
var mu sync.Mutex

func initDB() {
	var err error
	connStr := "postgres://sudoku:sudoku_secret@192.168.1.163:5433/sudokudb?sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		panic("DB 연결 실패: " + err.Error())
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			username VARCHAR(50) PRIMARY KEY,
			password VARCHAR(255) NOT NULL
		);
	`)
	if err != nil {
		panic("users 테이블 생성 실패: " + err.Error())
	}

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
	initDB()
	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	r.POST("/api/login", func(c *gin.Context) {
		var creds Credentials
		if err := c.BindJSON(&creds); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

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

	r.POST("/api/save", func(c *gin.Context) {
		var req SaveRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 데이터 형식입니다."})
			return
		}

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

		c.Data(http.StatusOK, "application/json", []byte(gameData))
	})

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

			// 👇 [핵심 수정] UnixNano()를 자바스크립트가 버틸 수 있는 UnixMilli()로 변경! (桁数削減)
			seed := time.Now().UnixMilli()
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
