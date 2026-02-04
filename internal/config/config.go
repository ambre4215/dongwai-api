package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DB_DSN            string
	JWT_SECRET        string
	PORT              string
	DEEPSEEK_API_KEY  string // 新增
	DEEPSEEK_BASE_URL string // 新增
}

var AppConfig *Config

func LoadConfig() {
	_ = godotenv.Load()

	AppConfig = &Config{
		DB_DSN:            getEnv("DB_DSN", "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable TimeZone=Asia/Shanghai"),
		JWT_SECRET:        getEnv("JWT_SECRET", "default_secret"),
		PORT:              getEnv("PORT", "8080"),
		DEEPSEEK_API_KEY:  getEnv("DEEPSEEK_API_KEY", ""),
		DEEPSEEK_BASE_URL: getEnv("DEEPSEEK_BASE_URL", "https://api.deepseek.com"), // 默认官方地址
	}

	if AppConfig.DEEPSEEK_API_KEY == "" {
		log.Println("⚠️以此警告: 未配置 DEEPSEEK_API_KEY，多义词消歧功能将不可用！")
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
