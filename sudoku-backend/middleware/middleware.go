package middleware

import (
	"net/http"
	"os"
	"strings"
	"time"

	"sync"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// ─────────────────────────────────────
// JWT 미들웨어
// ─────────────────────────────────────

type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

var jwtKey = []byte(getEnv("JWT_SECRET", "sudoku_secret_key_123"))

func JWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "인증 토큰이 없습니다."})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "유효하지 않은 토큰입니다."})
			return
		}

		// 이후 핸들러에서 c.GetInt("userID"), c.GetString("username") 으로 꺼냄
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// JWT 서명 함수 (handlers 패키지에서 사용)
func SignAccessToken(userID int, username string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)), // ✅ Access: 15분
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtKey)
}

// ─────────────────────────────────────
// ✅ Rate Limiter (IP별 로그인 시도 제한)
// ─────────────────────────────────────
// 같은 IP에서 1초에 5번 이상 요청하면 429 반환
// Brute-force 공격 방어용

type ipLimiter struct {
	limiters sync.Map
}

var globalLimiter = &ipLimiter{}

func (il *ipLimiter) get(ip string) *rate.Limiter {
	val, _ := il.limiters.LoadOrStore(ip, rate.NewLimiter(rate.Every(time.Second), 5))
	return val.(*rate.Limiter)
}

func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !globalLimiter.get(ip).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "요청이 너무 많습니다. 잠시 후 다시 시도하세요.",
			})
			return
		}
		c.Next()
	}
}

// ─────────────────────────────────────
// zerolog 기반 요청 로거
// ─────────────────────────────────────

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		log.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency", time.Since(start)).
			Str("ip", c.ClientIP()).
			Msg("request")
	}
}

// ─────────────────────────────────────
// 패닉 복구
// ─────────────────────────────────────

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error().Interface("panic", err).Msg("패닉 복구")
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "서버 내부 오류가 발생했습니다.",
				})
			}
		}()
		c.Next()
	}
}

// ─────────────────────────────────────
// CORS (Flutter 앱 허용)
// ─────────────────────────────────────

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
