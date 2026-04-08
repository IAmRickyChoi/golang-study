package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWT 서명을 위한 시크릿 키 (보안을 위해 실제 운영 시에는 환경 변수(環境変数)로 분리해야 합니다)
var jwtKey = []byte("sudoku_secret_key_123")

// 클라이언트가 보낼 로그인 정보 구조체 (構造体)
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// JWT에 담을 페이로드(ペイロード) 데이터
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// 임시 인메모리 유저 데이터 (추후 MySQL 등 DB로 교체 예정)
var users = map[string]string{
	"ricky": "password123",
}

func main() {
	r := gin.Default()

	// 로그인 (ログイン) API 엔드포인트
	r.POST("/api/login", func(c *gin.Context) {
		var creds Credentials

		// JSON 요청을 구조체로 바인딩 (バインディング)
		if err := c.BindJSON(&creds); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청입니다"})
			return
		}

		// 유저 검증 (ユーザー検証)
		expectedPassword, ok := users[creds.Username]
		if !ok || expectedPassword != creds.Password {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "인증 실패"})
			return
		}

		// JWT 토큰 만료 시간 설정 (24시간)
		expirationTime := time.Now().Add(24 * time.Hour)
		claims := &Claims{
			Username: creds.Username,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expirationTime),
			},
		}

		// 토큰 생성 및 서명 (署名)
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(jwtKey)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "토큰 생성 실패"})
			return
		}

		// 성공 시 토큰 반환 (レスポンス)
		c.JSON(http.StatusOK, gin.H{"token": tokenString, "username": creds.Username})
	})

	// 8080 포트에서 서버 실행 (サーバー起動)
	r.Run(":8080")
}
