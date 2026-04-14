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

type RecordRequest struct {
	Username string `json:"username"`
	Result   string `json:"result"`
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type MatchClient struct {
	conn     *websocket.Conn
	opponent *MatchClient
}

var waitingMatchClient *MatchClient
var matchMu sync.Mutex

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
			password VARCHAR(255) NOT NULL,
			wins INT DEFAULT 0,
			losses INT DEFAULT 0
		);
	`)
	if err != nil {
		panic("users 테이블 생성 실패: " + err.Error())
	}

	db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS wins INT DEFAULT 0;`)
	db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS losses INT DEFAULT 0;`)

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

	r.POST("/api/register", func(c *gin.Context) {
		var creds Credentials
		if err := c.BindJSON(&creds); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		var exists bool
		db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", creds.Username).Scan(&exists)
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "이미 사용 중인 아이디입니다."})
			return
		}

		_, err := db.Exec("INSERT INTO users (username, password, wins, losses) VALUES ($1, $2, 0, 0)", creds.Username, creds.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "가입 실패"})
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
		tokenString, _ := token.SignedString(jwtKey)

		c.JSON(http.StatusOK, gin.H{"token": tokenString, "username": creds.Username})
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
		tokenString, _ := token.SignedString(jwtKey)

		c.JSON(http.StatusOK, gin.H{"token": tokenString, "username": creds.Username})
	})

	r.POST("/api/record", func(c *gin.Context) {
		var req RecordRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 데이터 형식입니다."})
			return
		}

		if req.Result == "win" {
			db.Exec("UPDATE users SET wins = wins + 1 WHERE username = $1", req.Username)
		} else if req.Result == "loss" {
			db.Exec("UPDATE users SET losses = losses + 1 WHERE username = $1", req.Username)
		}
		c.JSON(http.StatusOK, gin.H{"message": "전적 기록 완료"})
	})

	r.GET("/api/stats", func(c *gin.Context) {
		username := c.Query("username")
		var wins, losses int
		err := db.QueryRow("SELECT wins, losses FROM users WHERE username = $1", username).Scan(&wins, &losses)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"wins": 0, "losses": 0})
			return
		}
		c.JSON(http.StatusOK, gin.H{"wins": wins, "losses": losses})
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
		db.Exec(query, req.Username, req.Data)
		c.JSON(http.StatusOK, gin.H{"message": "성공적으로 저장되었습니다."})
	})

	r.GET("/api/load", func(c *gin.Context) {
		username := c.Query("username")
		var gameData string
		err := db.QueryRow("SELECT game_data FROM saved_games WHERE username = $1", username).Scan(&gameData)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "저장된 게임이 없습니다."})
			return
		}
		c.Data(http.StatusOK, "application/json", []byte(gameData))
	})

	r.GET("/ws/match", func(c *gin.Context) {
		username := c.Query("username")
		if username == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
			return
		}

		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		client := &MatchClient{conn: ws}

		matchMu.Lock()
		if waitingMatchClient == nil {
			waitingMatchClient = client
			matchMu.Unlock()

			ws.WriteJSON(gin.H{"type": "waiting", "message": "상대방을 기다리는 중..."})

			for {
				msgType, msg, err := ws.ReadMessage()
				if err != nil {
					matchMu.Lock()
					if waitingMatchClient == client {
						waitingMatchClient = nil
					}
					matchMu.Unlock()

					// 👇 [수정] 무단으로 패배를 추가하던 코드를 지웠습니다!
					if client.opponent != nil {
						client.opponent.conn.WriteJSON(gin.H{"type": "opponent_left"})
						time.Sleep(100 * time.Millisecond)
						client.opponent.conn.Close()
					}
					break
				}
				if client.opponent != nil {
					client.opponent.conn.WriteMessage(msgType, msg)
				}
			}
		} else {
			opponent := waitingMatchClient
			waitingMatchClient = nil
			matchMu.Unlock()

			client.opponent = opponent
			opponent.opponent = client

			seed := time.Now().UnixMilli()
			matchData := gin.H{"type": "matched", "seed": seed, "difficulty": "medium"}

			opponent.conn.WriteJSON(matchData)
			ws.WriteJSON(matchData)

			for {
				msgType, msg, err := ws.ReadMessage()
				if err != nil {
					// 👇 [수정] 여기서도 패배 강제 추가 로직을 지웠습니다!
					if client.opponent != nil {
						client.opponent.conn.WriteJSON(gin.H{"type": "opponent_left"})
						time.Sleep(100 * time.Millisecond)
						client.opponent.conn.Close()
					}
					break
				}
				if client.opponent != nil {
					client.opponent.conn.WriteMessage(msgType, msg)
				}
			}
		}
	})

	fmt.Println("서버가 8080 포트에서 실행 중입니다...")
	r.Run(":8080")
}
