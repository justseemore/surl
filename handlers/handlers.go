package handlers

import (
	"log"
	"strconv"
	"time" // 添加 time 包导入

	"github.com/gofiber/fiber/v2"
	"github.com/justseemore/surl/config"
	"github.com/justseemore/surl/middleware"
	"github.com/justseemore/surl/services"
)

type Handler struct {
	urlService  *services.URLService
	authService *services.AuthService
	config      *config.Config
}

func NewHandler(urlService *services.URLService, authService *services.AuthService, config *config.Config) *Handler {
	return &Handler{
		urlService:  urlService,
		authService: authService,
		config:      config,
	}
}

// 登录页面
func (h *Handler) LoginPage(c *fiber.Ctx) error {
	return c.Render("login", fiber.Map{
		"title": "管理员登录",
	})
}

// 登录处理
func (h *Handler) Login(c *fiber.Ctx) error {
	type LoginRequest struct {
		Username string `json:"username" form:"username"`
		Password string `json:"password" form:"password"`
	}

	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "无效的请求格式",
		})
	}

	user, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	token, err := h.authService.GenerateToken(user)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "生成令牌失败",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"token":   token,
		"user": fiber.Map{
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

// CreateShortURL 创建短链接（仅限认证用户）
func (h *Handler) CreateShortURL(c *fiber.Ctx) error {
	type CreateRequest struct {
		OriginalURL string     `json:"original_url" form:"original_url"`
		Title       string     `json:"title" form:"title"`
		Description string     `json:"description" form:"description"`
		ExpiresAt   *time.Time `json:"expires_at" form:"expires_at"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "无效的请求格式",
		})
	}

	if req.OriginalURL == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "原始链接不能为空",
		})
	}

	// 从JWT中获取用户名（修复：使用username而不是user_id）
	username := c.Locals("username").(string)

	// 修复：传递username作为createdBy参数
	shortURL, err := h.urlService.CreateShortURL(req.OriginalURL, req.Title, req.Description, h.config.CustomDomain, req.ExpiresAt, username)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "创建短链接失败: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"short_url":  "https://" + h.config.CustomDomain + "/" + shortURL.ShortCode,
		"short_code": shortURL.ShortCode,
		"qr_code":    "data:image/svg+xml;base64," + shortURL.ShortCode,
	})
}

// Admin 管理员页面
func (h *Handler) Admin(c *fiber.Ctx) error {
	return c.Render("admin", fiber.Map{
		"title": "后台管理",
	})
}

// GetURLs API获取URL列表（需要认证）
func (h *Handler) GetURLs(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	search := c.Query("search", "")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// 修复：添加createdBy参数
	username := c.Locals("username").(string)
	role := c.Locals("role").(string)
	createdBy := ""
	if role != "admin" {
		createdBy = username // 非管理员只能看自己的记录
	}

	urls, total, err := h.urlService.GetURLList(page, limit, search, createdBy)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "获取数据失败",
		})
	}

	// 计算总页数
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	return c.JSON(fiber.Map{
		"urls":         urls,
		"total":        total,
		"current_page": page,
		"total_pages":  totalPages,
		"limit":        limit,
		"success":      true,
	})
}

// UpdateURL 更新URL（需要认证）
func (h *Handler) UpdateURL(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "无效的ID",
		})
	}

	type UpdateRequest struct {
		OriginalURL string     `json:"original_url"`
		Title       string     `json:"title"`
		ExpiresAt   *time.Time `json:"expires_at"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "无效的请求格式",
		})
	}

	// 修复：添加updatedBy参数
	username := c.Locals("username").(string)
	err = h.urlService.UpdateURL(uint(id), req.OriginalURL, req.Title, req.ExpiresAt, username)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "更新失败: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "更新成功",
	})
}

// DeleteURL 删除URL（需要认证）
func (h *Handler) DeleteURL(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "无效的ID",
		})
	}

	// 修复：添加deletedBy参数
	username := c.Locals("username").(string)
	err = h.urlService.DeleteURL(uint(id), username)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "删除失败: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "删除成功",
	})
}

// GetURLByID 根据ID获取单个URL
func (h *Handler) GetURLByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "无效的ID",
		})
	}

	url, err := h.urlService.GetURLByID(uint(id))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"url":     url,
	})
}

// BatchDeleteURLs 批量删除URLs
func (h *Handler) BatchDeleteURLs(c *fiber.Ctx) error {
	type BatchDeleteRequest struct {
		IDs []uint `json:"ids"`
	}

	var req BatchDeleteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "无效的请求格式",
		})
	}
	username := c.Locals("username").(string)
	err := h.urlService.BatchDeleteURLs(req.IDs, username)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "批量删除失败: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "批量删除成功",
	})
}

// GetStats 获取统计信息
func (h *Handler) GetStats(c *fiber.Ctx) error {
	username := c.Locals("username").(string)
	stats, err := h.urlService.GetURLStats(username)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "获取统计信息失败",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"stats":   stats,
	})
}

// CleanupExpired 清理过期链接
func (h *Handler) CleanupExpired(c *fiber.Ctx) error {
	err := h.urlService.CleanupExpiredURLs()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "清理过期链接失败: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "清理完成",
	})
}

// GetExpiredURLs 获取过期链接列表
func (h *Handler) GetExpiredURLs(c *fiber.Ctx) error {
	urls, err := h.urlService.GetExpiredURLs()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "获取过期链接失败",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"urls":    urls,
	})
}

// GetProfile 获取用户信息
func (h *Handler) GetProfile(c *fiber.Ctx) error {
	username := c.Locals("username").(string)
	accountInfo := h.authService.GetAccountInfo(username)
	if accountInfo == nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "用户不存在",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"user":    accountInfo,
	})
}

// Logout 注销登录
func (h *Handler) Logout(c *fiber.Ctx) error {
	// 由于使用JWT，服务端无状态，客户端删除token即可
	return c.JSON(fiber.Map{
		"success": true,
		"message": "注销成功",
	})
}

// Redirect 处理短代码重定向
func (h *Handler) Redirect(c *fiber.Ctx) error {
	shortCode := c.Params("code")
	if shortCode == "" {
		return c.Status(404).SendString("短代码不能为空")
	}

	// 获取URL信息
	url, err := h.urlService.GetURLByShortCode(shortCode)
	if err != nil {
		return c.Status(404).SendString("短链接不存在或已过期")
	}

	// 检查是否激活
	if !url.IsActive {
		return c.Status(404).SendString("短链接已禁用")
	}
	// 增加点击计数
	h.urlService.IncrementClickCount(shortCode)
	// 获取UA信息
	uaInfo := c.Locals("uaInfo")
	if uaInfo != nil {
		ua := uaInfo.(*middleware.UAInfo)
		// 如果是微信或QQ访问，跳转到拦截页面
		if ua.NeedsBlock {
			return c.Render("block", fiber.Map{
				"title":       "链接跳转提示",
				"originalURL": url.OriginalURL,
				"title_text":  url.Title,
				"isWeChat":    ua.IsWeChat,
				"isQQ":        ua.IsQQ,
			})
		}
	}

	// 记录点击（修复：传递完整的 URL 对象而不是 ID）
	// h.urlService.RecordClick(url, c.Get("User-Agent"), c.IP(), c.Get("Referer"))

	// 直接重定向
	return c.Redirect(url.OriginalURL, 302)
}

// Index 主页
func (h *Handler) Index(c *fiber.Ctx) error {
	return c.Render("index", fiber.Map{
		"title": "短链接服务",
	})
}

// GenerateQRCode 生成二维码（占位符，需要具体实现）
func (h *Handler) GenerateQRCode(c *fiber.Ctx) error {
	shortCode := c.Params("code")
	if shortCode == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "短代码不能为空",
		})
	}

	// 这里可以集成二维码生成库
	// 目前返回占位符响应
	return c.JSON(fiber.Map{
		"success":   true,
		"shortCode": shortCode,
		"qrcode":    "data:image/svg+xml;base64," + shortCode, // 占位符
	})
}

// BatchToggleURLs 批量切换URL状态
func (h *Handler) BatchToggleURLs(c *fiber.Ctx) error {
	type BatchToggleRequest struct {
		IDs    []uint `json:"ids"`
		Active bool   `json:"active"`
	}

	var req BatchToggleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "无效的请求格式",
		})
	}

	if len(req.IDs) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"error": "请选择要操作的URL",
		})
	}
	log.Printf("批量切换URL状态: %v, %v", req.IDs, req.Active)
	username := c.Locals("username").(string)
	err := h.urlService.BatchToggleURLs(req.IDs, req.Active, username)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "批量切换状态失败",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "批量操作完成",
	})
}

// GetClickStats 获取点击统计
// func (h *Handler) GetClickStats(c *fiber.Ctx) error {
// 	shortCode := c.Params("code")
// 	if shortCode == "" {
// 		return c.Status(400).JSON(fiber.Map{
// 			"error": "短代码不能为空",
// 		})
// 	}

// 	// 获取天数参数，默认7天
// 	days := c.QueryInt("days", 7)
// 	if days < 1 || days > 365 {
// 		days = 7
// 	}

// 	// 获取URL信息
// 	url, err := h.urlService.GetURLByShortCode(shortCode)
// 	if err != nil {
// 		return c.Status(404).JSON(fiber.Map{
// 			"error": "短链接不存在",
// 		})
// 	}

// 	// 获取点击统计
// 	stats, err := h.urlService.GetClickStats(url.ID, days)
// 	if err != nil {
// 		return c.Status(500).JSON(fiber.Map{
// 			"error": "获取统计信息失败",
// 		})
// 	}

// 	return c.JSON(fiber.Map{
// 		"success": true,
// 		"stats":   stats,
// 		"url":     url,
// 	})
// }
