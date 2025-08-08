package services

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/justseemore/surl/config"
)

type AuthService struct {
	config    *config.Config
	jwtSecret []byte
}

// AuthUser 结构体
type AuthUser struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

// NewAuthService 创建认证服务实例
func NewAuthService(cfg *config.Config) *AuthService {
	return &AuthService{
		config:    cfg,
		jwtSecret: []byte(cfg.JWTSecret),
	}
}

// Login 用户登录验证
func (s *AuthService) Login(username, password string) (*AuthUser, error) {
	// 在配置的账户列表中查找匹配的用户
	for _, account := range s.config.Accounts {
		if account.Username == username && account.Password == password {
			return &AuthUser{
				Username: account.Username,
				Role:     account.Role,
			}, nil
		}
	}

	return nil, errors.New("用户名或密码错误")
}

// GenerateToken 生成JWT令牌
func (s *AuthService) GenerateToken(user *AuthUser) (string, error) {
	claims := jwt.MapClaims{
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(time.Hour * 24).Unix(), // 24小时有效期
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ValidateToken 验证JWT令牌
func (s *AuthService) ValidateToken(tokenString string) (*AuthUser, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("无效的令牌")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("无效的令牌声明")
	}

	username, ok := claims["username"].(string)
	if !ok {
		return nil, errors.New("令牌中缺少用户名")
	}

	role, ok := claims["role"].(string)
	if !ok {
		return nil, errors.New("令牌中缺少角色信息")
	}

	return &AuthUser{
		Username: username,
		Role:     role,
	}, nil
}

// IsAdmin 检查用户是否为管理员
func (s *AuthService) IsAdmin(user *AuthUser) bool {
	return user.Role == "admin"
}

// GetAccountInfo 获取账户信息（不返回密码）
func (s *AuthService) GetAccountInfo(username string) *config.Account {
	for _, account := range s.config.Accounts {
		if account.Username == username {
			return &config.Account{
				Username: account.Username,
				Role:     account.Role,
				Password: "", // 不返回密码
			}
		}
	}
	return nil
}
