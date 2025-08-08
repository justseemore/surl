package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"

	"github.com/justseemore/surl/cache"
	"github.com/justseemore/surl/config"
	"github.com/justseemore/surl/handlers"
	"github.com/justseemore/surl/middleware"
	"github.com/justseemore/surl/models"
	"github.com/justseemore/surl/services"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 设置JWT密钥
	middleware.SetJWTSecret(cfg.JWTSecret)

	// 初始化数据库
	if err := models.InitDatabase(cfg.DBPath); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// 初始化服务 - 使用带内存限制的缓存管理器
	cacheManager := cache.NewCacheManager(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.CacheExpiry, cfg.CacheMaxItems)
	urlService := services.NewURLService(cacheManager, models.DB, cfg)
	authService := services.NewAuthService(cfg)

	// 启动异步任务
	go urlService.StartClickCountSync()

	// 启动缓存预热
	urlService.WarmupCache()

	// 创建模板引擎
	engine := html.New("./templates", ".html")
	// engine.AddFunc("sub", func(a, b int) int { return a - b })
	// engine.AddFunc("add", func(a, b int) int { return a + b })
	// engine.AddFunc("div", func(a, b int) int { return a / b })

	// 创建Fiber应用
	app := fiber.New(fiber.Config{
		Views:   engine,
		Prefork: true,
	})

	// 中间件
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New())
	app.Use(middleware.UADetector())

	// 静态文件
	// app.Static("/static", "./static")

	// 初始化处理器
	handler := handlers.NewHandler(urlService, authService, cfg)

	// 设置路由
	setupRoutes(app, handler)

	// 启动服务器
	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			log.Fatal("Failed to start server:", err)
		}
	}()

	log.Printf("Server started on port %s", cfg.Port)

	// 优雅关闭
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Fatal("Server shutdown failed:", err)
	}

	log.Println("Server shutdown complete")
}

func setupRoutes(app *fiber.App, handler *handlers.Handler) {
	// 添加UA检测中间件到需要检测的路由
	app.Use(middleware.UADetector())
	app.Get("/", handler.Index)
	// 公开路由
	app.Get("/login", handler.LoginPage)
	app.Post("/api/login", handler.Login)
	// 需要认证的管理路由
	admin := app.Group("/admin")
	admin.Get("/", handler.Admin)
	// 需要认证的API路由组
	api := app.Group("/api")
	api.Use(middleware.JWTMiddleware())
	// URL基础操作
	api.Post("/create", handler.CreateShortURL)
	api.Get("/urls", handler.GetURLs)
	api.Get("/urls/:id<int>", handler.GetURLByID) // 新增：根据ID获取单个URL
	api.Post("/urls/:id<int>/update", handler.UpdateURL)
	api.Post("/urls/:id<int>/delete", handler.DeleteURL)

	// 批量操作
	api.Post("/urls/batch/delete", handler.BatchDeleteURLs) // 新增：批量删除URLs
	api.Post("/urls/batch/toggle", handler.BatchToggleURLs) // 新增：批量切换URL状态

	// 统计相关
	api.Get("/stats", handler.GetStats) // 新增：获取统计信息

	// 清理操作
	api.Post("/cleanup/expired", handler.CleanupExpired) // 新增：清理过期链接
	api.Get("/expired", handler.GetExpiredURLs)          // 新增：获取过期链接列表

	// 用户相关
	api.Get("/profile", handler.GetProfile) // 新增：获取用户信息

	// 二维码生成
	api.Get("/qrcode/:code", handler.GenerateQRCode) // 新增：生成二维码

	// 重定向路由（放在最后以避免冲突）
	app.Get("/:code", handler.Redirect)
}
