package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"sudoku-backend/database"
	"sudoku-backend/middleware"
)

type AuthHandler struct{}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

// ─────────────────────────────────────
// 요청/응답 구조체
// ─────────────────────────────────────

type registerRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20"`
	Password string `json:"password" binding:"required,min=6"`
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Username     string `json:"username"`
}

// ─────────────────────────────────────
// [POST /api/register] 회원가입
// ─────────────────────────────────────

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "아이디(3~20자), 비밀번호(6자 이상)를 입력하세요."})
		return
	}

	// 중복 아이디 확인
	var exists bool
	err := database.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", req.Username,
	).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "서버 오류"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "이미 사용 중인 아이디입니다."})
		return
	}

	// 비밀번호 해싱
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "서버 오류"})
		return
	}

	var userID int
	err = database.DB.QueryRow(
		"INSERT INTO users (username, password) VALUES ($1, $2) RETURNING id",
		req.Username, string(hashed),
	).Scan(&userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "회원가입 실패"})
		return
	}

	// 가입 후 바로 로그인 처리
	resp, err := issueTokens(userID, req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "토큰 발급 실패"})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// ─────────────────────────────────────
// [POST /api/login] 로그인
// ─────────────────────────────────────

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청입니다."})
		return
	}

	var userID int
	var dbPassword string
	err := database.DB.QueryRow(
		"SELECT id, password FROM users WHERE username = $1", req.Username,
	).Scan(&userID, &dbPassword)

	if err == sql.ErrNoRows {
		// ✅ 아이디/비밀번호 오류 메시지를 동일하게 → 어느 쪽이 틀렸는지 노출 방지
		c.JSON(http.StatusUnauthorized, gin.H{"error": "아이디 또는 비밀번호가 올바르지 않습니다."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "서버 오류"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(dbPassword), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "아이디 또는 비밀번호가 올바르지 않습니다."})
		return
	}

	resp, err := issueTokens(userID, req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "토큰 발급 실패"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ─────────────────────────────────────
// [POST /api/refresh] Access Token 재발급
// ─────────────────────────────────────
// Flutter 앱에서 access_token 만료 시 refresh_token으로 자동 재발급

func (h *AuthHandler) Refresh(c *gin.Context) {
	var body struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token이 필요합니다."})
		return
	}

	// DB에서 refresh_token 검증
	var userID int
	var username string
	var expiresAt time.Time

	err := database.DB.QueryRow(`
		SELECT u.id, u.username, rt.expires_at
		FROM refresh_tokens rt
		JOIN users u ON u.id = rt.user_id
		WHERE rt.token = $1
	`, body.RefreshToken).Scan(&userID, &username, &expiresAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "유효하지 않은 refresh token입니다."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "서버 오류"})
		return
	}

	// 만료 확인
	if time.Now().After(expiresAt) {
		// 만료된 토큰은 DB에서 삭제
		database.DB.Exec("DELETE FROM refresh_tokens WHERE token = $1", body.RefreshToken)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token이 만료되었습니다. 다시 로그인하세요."})
		return
	}

	// ✅ Token Rotation: 기존 refresh_token 삭제하고 새 토큰 발급
	// 탈취된 refresh_token이 재사용되는 것을 방지
	database.DB.Exec("DELETE FROM refresh_tokens WHERE token = $1", body.RefreshToken)

	resp, err := issueTokens(userID, username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "토큰 발급 실패"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ─────────────────────────────────────
// [POST /api/logout] 로그아웃
// ─────────────────────────────────────

func (h *AuthHandler) Logout(c *gin.Context) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	c.ShouldBindJSON(&body)

	if body.RefreshToken != "" {
		// refresh_token을 DB에서 삭제 → 이후 재발급 불가
		database.DB.Exec(
			"DELETE FROM refresh_tokens WHERE token = $1", body.RefreshToken,
		)
	}

	c.JSON(http.StatusOK, gin.H{"message": "로그아웃 되었습니다."})
}

// ─────────────────────────────────────
// 내부 헬퍼: Access + Refresh 토큰 발급
// ─────────────────────────────────────

func issueTokens(userID int, username string) (*tokenResponse, error) {
	// Access Token (15분)
	accessToken, err := middleware.SignAccessToken(userID, username)
	if err != nil {
		return nil, err
	}

	// Refresh Token: 암호학적으로 안전한 랜덤 32바이트 문자열
	refreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, err
	}

	// Refresh Token DB에 저장 (유효기간 30일)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	_, err = database.DB.Exec(
		"INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)",
		userID, refreshToken, expiresAt,
	)
	if err != nil {
		return nil, err
	}

	return &tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Username:     username,
	}, nil
}

func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
