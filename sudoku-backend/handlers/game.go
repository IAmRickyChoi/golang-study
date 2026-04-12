package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"sudoku-backend/database"
)

type GameHandler struct{}

func NewGameHandler() *GameHandler {
	return &GameHandler{}
}

// 슬롯 메타데이터 (목록 조회용)
type slotMeta struct {
	Slot      int       `json:"slot"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ─────────────────────────────────────
// [POST /api/save/:slot] 게임 저장
// ─────────────────────────────────────

func (h *GameHandler) Save(c *gin.Context) {
	userID, slot, ok := extractUserSlot(c)
	if !ok {
		return
	}

	// ✅ Flutter에서 보내는 JSON을 raw로 받아서 JSONB에 그대로 저장
	var body any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 데이터 형식입니다."})
		return
	}

	query := `
		INSERT INTO saved_games (user_id, slot, game_data, updated_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id, slot)
		DO UPDATE SET game_data = EXCLUDED.game_data, updated_at = CURRENT_TIMESTAMP
	`
	if _, err := database.DB.Exec(query, userID, slot, body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "저장 실패"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "저장 완료", "slot": slot})
}

// ─────────────────────────────────────
// [GET /api/load/:slot] 게임 불러오기
// ─────────────────────────────────────

func (h *GameHandler) Load(c *gin.Context) {
	userID, slot, ok := extractUserSlot(c)
	if !ok {
		return
	}

	var gameData []byte
	err := database.DB.QueryRow(
		"SELECT game_data FROM saved_games WHERE user_id = $1 AND slot = $2",
		userID, slot,
	).Scan(&gameData)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "해당 슬롯에 저장된 게임이 없습니다."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "불러오기 실패"})
		return
	}

	// JSONB를 이중 직렬화 없이 그대로 반환
	c.Data(http.StatusOK, "application/json", gameData)
}

// ─────────────────────────────────────
// [GET /api/slots] 내 세이브 슬롯 목록
// ─────────────────────────────────────

func (h *GameHandler) ListSlots(c *gin.Context) {
	userID := c.GetInt("userID")

	rows, err := database.DB.Query(
		"SELECT slot, updated_at FROM saved_games WHERE user_id = $1 ORDER BY slot",
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "목록 조회 실패"})
		return
	}
	defer rows.Close()

	slots := []slotMeta{}
	for rows.Next() {
		var s slotMeta
		if err := rows.Scan(&s.Slot, &s.UpdatedAt); err != nil {
			continue
		}
		slots = append(slots, s)
	}

	c.JSON(http.StatusOK, gin.H{"slots": slots})
}

// ─────────────────────────────────────
// [DELETE /api/save/:slot] 세이브 삭제
// ─────────────────────────────────────

func (h *GameHandler) Delete(c *gin.Context) {
	userID, slot, ok := extractUserSlot(c)
	if !ok {
		return
	}

	result, err := database.DB.Exec(
		"DELETE FROM saved_games WHERE user_id = $1 AND slot = $2",
		userID, slot,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "삭제 실패"})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "해당 슬롯에 저장된 데이터가 없습니다."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "삭제 완료", "slot": slot})
}

// ─────────────────────────────────────
// 헬퍼: JWT에서 userID, URL에서 slot 추출
// ─────────────────────────────────────

func extractUserSlot(c *gin.Context) (userID, slot int, ok bool) {
	userID = c.GetInt("userID")

	slot, err := strconv.Atoi(c.Param("slot"))
	if err != nil || slot < 0 || slot > 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "슬롯 번호는 0~4 사이여야 합니다."})
		return 0, 0, false
	}

	return userID, slot, true
}
