package middleware

import (
    "strings"
    "github.com/gofiber/fiber/v2"
)

type UAInfo struct {
    IsMobile   bool
    IsTablet   bool
    IsDesktop  bool
    Browser    string
    OS         string
    Device     string
    IsWeChat   bool  // 新增：微信检测
    IsQQ       bool  // 新增：QQ检测
    NeedsBlock bool  // 新增：是否需要拦截
}

func UADetector() fiber.Handler {
    return func(c *fiber.Ctx) error {
        userAgent := strings.ToLower(c.Get("User-Agent"))
        
        uaInfo := &UAInfo{
            Browser: "Unknown",
            OS:      "Unknown",
            Device:  "Unknown",
        }
        
        // 检测微信和QQ
        if strings.Contains(userAgent, "micromessenger") {
            uaInfo.IsWeChat = true
            uaInfo.NeedsBlock = true
        }
        if strings.Contains(userAgent, "qq/") || strings.Contains(userAgent, "qqbrowser") {
            uaInfo.IsQQ = true
            uaInfo.NeedsBlock = true
        }
        
        // 检测设备类型
        if strings.Contains(userAgent, "mobile") || strings.Contains(userAgent, "android") || strings.Contains(userAgent, "iphone") {
            uaInfo.IsMobile = true
            uaInfo.Device = "Mobile"
        } else if strings.Contains(userAgent, "tablet") || strings.Contains(userAgent, "ipad") {
            uaInfo.IsTablet = true
            uaInfo.Device = "Tablet"
        } else {
            uaInfo.IsDesktop = true
            uaInfo.Device = "Desktop"
        }
        
        // 检测浏览器
        if strings.Contains(userAgent, "chrome") {
            uaInfo.Browser = "Chrome"
        } else if strings.Contains(userAgent, "firefox") {
            uaInfo.Browser = "Firefox"
        } else if strings.Contains(userAgent, "safari") {
            uaInfo.Browser = "Safari"
        } else if strings.Contains(userAgent, "edge") {
            uaInfo.Browser = "Edge"
        }
        
        // 检测操作系统
        if strings.Contains(userAgent, "windows") {
            uaInfo.OS = "Windows"
        } else if strings.Contains(userAgent, "mac") {
            uaInfo.OS = "macOS"
        } else if strings.Contains(userAgent, "linux") {
            uaInfo.OS = "Linux"
        } else if strings.Contains(userAgent, "android") {
            uaInfo.OS = "Android"
        } else if strings.Contains(userAgent, "ios") {
            uaInfo.OS = "iOS"
        }
        
        // 将UA信息存储到上下文中
        c.Locals("uaInfo", uaInfo)
        
        return c.Next()
    }
}