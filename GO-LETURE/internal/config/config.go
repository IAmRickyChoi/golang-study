package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	DBTimeZone string
}

// Load: .env를 읽어서 Config 구조체로 반환
func Load() *Config {
	// .env가 없어도 에러로 멈추지 않음 (배포 서버에서는 OS 환경변수를 쓰기 때문)
	if err := godotenv.Load(); err != nil {
		log.Println(".env 파일 없음 → 시스템 환경변수 사용")
	}

	return &Config{
		ServerPort: getEnv("SERVER_PORT", "9900"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     mustGetEnv("DB_USER"),
		DBPassword: mustGetEnv("DB_PASSWORD"),
		DBName:     getEnv("DB_NAME", "mydb"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),
		DBTimeZone: getEnv("DB_TIMEZONE", "Asia/Tokyo"),
	}
}

// DSN: 흩어진 값을 조립해서 접속 문자열로 만들어 줌
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		c.DBHost, c.DBUser, c.DBPassword, c.DBName, c.DBPort, c.DBSSLMode, c.DBTimeZone,
	)
}

// 값이 없으면 기본값 사용
func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// 값이 없으면 서버를 시작하지 않음 (비밀번호처럼 기본값이 있으면 안 되는 것)
func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("환경변수 %s 가 설정되지 않았습니다", key)
	}
	return v
}
