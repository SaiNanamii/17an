package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port       string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	DBConnMaxIdleTime time.Duration
}

func Load() *Config {
	godotenv.Load()

	return &Config{
		Port:       getEnv("PORT", "3000"),
		DBHost:     getEnv("DB_HOST", "127.0.0.1"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "lomba"),
		DBPassword: getEnv("DB_PASSWORD", "lomba"),
		DBName:     getEnv("DB_NAME", "lomba_challenge"),

		DBMaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 20),
		DBMaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
		DBConnMaxLifetime: getEnvMinutes("DB_CONN_MAX_LIFETIME_MINUTES", 30),
		DBConnMaxIdleTime: getEnvMinutes("DB_CONN_MAX_IDLE_TIME_MINUTES", 5),
	}
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable statement_timeout=20000 lock_timeout=3000",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvMinutes(key string, fallbackMinutes int) time.Duration {
	return time.Duration(getEnvInt(key, fallbackMinutes)) * time.Minute
}
