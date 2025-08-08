package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Account struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type Config struct {
	Port          string
	CustomDomain  string
	DBPath        string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	CacheExpiry   int // 分钟
	CacheMaxItems int // 新增：内存缓存最大项目数
	JWTSecret     string
	Accounts      []Account
	MaxURLLength  int
	DefaultExpiry int
}

func Load() *Config {
	// 尝试加载 .env 文件
	if err := godotenv.Load(); err != nil {
		// 如果 .env 文件不存在，继续使用环境变量
		log.Println("Warning: .env file not found, using environment variables")
	}

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	cacheExpiry, _ := strconv.Atoi(getEnv("CACHE_EXPIRY", "60"))
	cacheMaxItems, _ := strconv.Atoi(getEnv("CACHE_MAX_ITEMS", "10000")) // 新增
	maxURLLength, _ := strconv.Atoi(getEnv("MAX_URL_LENGTH", "2048"))
	defaultExpiry, _ := strconv.Atoi(getEnv("DEFAULT_EXPIRY", "8760")) // 1年

	// 解析账户配置
	accounts := parseAccounts()

	return &Config{
		Port:          getEnv("PORT", "3001"),
		CustomDomain:  getEnv("CUSTOM_DOMAIN", "localhost:3000"),
		DBPath:        getEnv("DB_PATH", "./surl.db"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""), // 新增Redis密码配置
		RedisDB:       redisDB,
		CacheExpiry:   cacheExpiry,
		CacheMaxItems: cacheMaxItems, // 新增
		JWTSecret:     getEnv("JWT_SECRET", "default_jwt_secret_change_in_production"),
		Accounts:      accounts,
		MaxURLLength:  maxURLLength,
		DefaultExpiry: defaultExpiry,
	}
}

// parseAccounts 解析账户配置
// 格式：ACCOUNTS=admin:password123:admin,user1:pass456:user
func parseAccounts() []Account {
	accountsStr := getEnv("ACCOUNTS", "admin:admin123:admin")
	var accounts []Account

	accountList := strings.Split(accountsStr, ",")
	for _, accountStr := range accountList {
		parts := strings.Split(strings.TrimSpace(accountStr), ":")
		if len(parts) >= 3 {
			accounts = append(accounts, Account{
				Username: parts[0],
				Password: parts[1],
				Role:     parts[2],
			})
		}
	}
	// 如果没有配置账户，创建默认管理员
	if len(accounts) == 0 {
		accounts = append(accounts, Account{
			Username: "admin",
			Password: "admin123",
			Role:     "admin",
		})
		log.Println("Warning: No accounts configured, using default admin account")
	}

	return accounts
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
