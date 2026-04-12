package database

import (
	"database/sql"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

var DB *sql.DB

func Init() error {
	connStr := getEnv(
		"DATABASE_URL",
		"postgres://sudoku:sudoku_secret@192.168.1.163:5433/sudokudb?sslmode=disable",
	)

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}

	// ✅ 커넥션 풀 설정
	// 트래픽이 몰려도 DB 연결이 고갈되지 않도록 제한
	DB.SetMaxOpenConns(25)                 // 최대 동시 연결 수
	DB.SetMaxIdleConns(10)                 // 유휴 연결 유지 수
	DB.SetConnMaxLifetime(5 * time.Minute) // 연결 최대 수명 (오래된 연결 자동 교체)
	DB.SetConnMaxIdleTime(1 * time.Minute) // 유휴 연결 최대 유지 시간

	// 실제 연결 확인
	if err := DB.Ping(); err != nil {
		return err
	}

	log.Info().Msg("DB 연결 성공")

	// 테이블 자동 생성
	return migrate()
}

func Close() {
	if DB != nil {
		DB.Close()
		log.Info().Msg("DB 연결 종료")
	}
}

// ✅ 마이그레이션: 테이블 생성 및 구조 정의
func migrate() error {
	queries := []string{
		// 유저 테이블
		`CREATE TABLE IF NOT EXISTS users (
			id         SERIAL PRIMARY KEY,
			username   VARCHAR(50)  UNIQUE NOT NULL,
			password   VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,

		// ✅ Refresh Token 테이블
		// 로그아웃 시 토큰을 삭제하고, 재발급 시 여기서 검증
		`CREATE TABLE IF NOT EXISTS refresh_tokens (
			id         SERIAL PRIMARY KEY,
			user_id    INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token      VARCHAR(512) UNIQUE NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,

		// ✅ 세이브 슬롯 테이블 (유저당 최대 5슬롯)
		`CREATE TABLE IF NOT EXISTS saved_games (
			id         SERIAL PRIMARY KEY,
			user_id    INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			slot       SMALLINT NOT NULL CHECK (slot BETWEEN 0 AND 4),
			game_data  JSONB NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (user_id, slot)
		);`,

		// 만료된 refresh_token 조회 성능을 위한 인덱스
		`CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token ON refresh_tokens(token);`,
		`CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires ON refresh_tokens(expires_at);`,
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			return err
		}
	}

	log.Info().Msg("DB 마이그레이션 완료")
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
