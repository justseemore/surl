package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/justseemore/surl/cache"
	"github.com/justseemore/surl/config"
	"github.com/justseemore/surl/models"
	"gorm.io/gorm"
)

type URLService struct {
	cacheManager *cache.Manager
	db           *gorm.DB
	config       *config.Config
}

type URLStats struct {
	TotalURLs   int64 `json:"total_urls"`
	ActiveURLs  int64 `json:"active_urls"`
	TotalClicks int64 `json:"total_clicks"`
	TodayClicks int64 `json:"today_clicks"`
}

func NewURLService(cacheManager *cache.Manager, db *gorm.DB, cfg *config.Config) *URLService {
	return &URLService{
		cacheManager: cacheManager,
		db:           db,
		config:       cfg,
	}
}

// generateShortCode 生成短代码
func (s *URLService) generateShortCode() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 8

	result := make([]byte, length)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

// validateURL 验证URL格式
func (s *URLService) validateURL(rawURL string) (string, error) {
	if rawURL == "" {
		return "", errors.New("URL不能为空")
	}

	// 检查URL长度
	if len(rawURL) > s.config.MaxURLLength {
		return "", fmt.Errorf("URL长度不能超过%d个字符", s.config.MaxURLLength)
	}

	// 验证URL格式
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("无效的URL格式: %v", err)
	}

	// 检查主机名
	if parsedURL.Host == "" {
		return "", errors.New("URL必须包含有效的主机名")
	}

	// 禁止本地地址（可选）
	if strings.Contains(parsedURL.Host, "localhost") || strings.Contains(parsedURL.Host, "127.0.0.1") {
		return "", errors.New("不允许使用本地地址")
	}

	return rawURL, nil
}

// CreateShortURL 创建短链接
func (s *URLService) CreateShortURL(originalURL, title, description, domain string, expiresAt *time.Time, createdBy string) (*models.URL, error) {
	// 验证URL
	validatedURL, err := s.validateURL(originalURL)
	if err != nil {
		return nil, err
	}

	// 检查URL是否已存在
	var existingURL models.URL
	if err := s.db.Where("original_url = ? AND deleted_at IS NULL", validatedURL).First(&existingURL).Error; err == nil {
		return nil, errors.New("URL已存在")
	}

	// 生成唯一短代码
	shortCode := s.generateShortCodeFromURL(validatedURL)
	// 设置默认过期时间
	if expiresAt == nil {
		defaultExpiry := time.Now().Add(time.Duration(s.config.DefaultExpiry) * time.Hour)
		expiresAt = &defaultExpiry
	}

	url := &models.URL{
		ShortCode:    shortCode,
		OriginalURL:  validatedURL,
		Title:        title,
		Description:  description,
		CustomDomain: domain,
		IsActive:     true,
		ExpiresAt:    expiresAt,
		CreatedBy:    createdBy,
	}

	if err := s.db.Create(url).Error; err != nil {
		return nil, fmt.Errorf("创建短链接失败: %v", err)
	}

	// 创建成功后，立即将新创建的URL加载到缓存中
	s.cacheManager.SetURL(shortCode, url)

	return url, nil
}

// GetURLByShortCode 根据短代码获取URL
func (s *URLService) GetURLByShortCode(shortCode string) (*models.URL, error) {
	// 首先尝试从缓存获取
	if url, found := s.cacheManager.GetURL(shortCode); found {
		// 检查缓存中的URL是否有效且未过期
		if url.IsActive && (url.ExpiresAt == nil || url.ExpiresAt.After(time.Now())) {
			return url, nil
		}
	}
	return nil, errors.New("链接已失效")
}

// GetURLList 获取URL列表
func (s *URLService) GetURLList(page, pageSize int, search, createdBy string) ([]models.URL, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var urls []models.URL
	var total int64

	query := s.db.Model(&models.URL{})

	// 按创建者过滤
	if createdBy != "" {
		query = query.Where("created_by = ?", createdBy)
	}

	// 搜索过滤
	if search != "" {
		query = query.Where("original_url ILIKE ? OR title ILIKE ? OR description ILIKE ? OR short_code ILIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	// 获取总数
	query.Count(&total)

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&urls).Error

	return urls, total, err
}

// GetURLStats 获取URL统计信息
func (s *URLService) GetURLStats(createdBy string) (*URLStats, error) {
	stats := &URLStats{}

	query := s.db.Model(&models.URL{})
	if createdBy != "" {
		query = query.Where("created_by = ?", createdBy)
	}
	// 总URL数
	query.Count(&stats.TotalURLs)
	// 活跃URL数
	query.Where("is_active = ? AND (expires_at IS NULL OR expires_at > ?)", true, time.Now()).Count(&stats.ActiveURLs)
	// 总点击数
	s.db.Model(&models.URL{}).Select("COALESCE(SUM(click_count), 0)").Where("created_by = ? OR ? = ''", createdBy, createdBy).Scan(&stats.TotalClicks)
	return stats, nil
}

// IncrementClickCount 增加点击计数
func (s *URLService) IncrementClickCount(shortCode string) {
	s.cacheManager.IncrementClickCount(shortCode)
}

// 移除RecordClick函数
// func (s *URLService) RecordClick(url *models.URL, userAgent, ip, referer string) {
//     ...
// }

// 移除parseUserAgent函数
// func parseUserAgent(userAgent string) (device, browser, os string) {
//     ...
// }

// UpdateURL 更新URL
func (s *URLService) UpdateURL(id uint, originalURL, title string, expiresAt *time.Time, active bool, updatedBy string) error {
	var url models.URL
	if err := s.db.First(&url, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("URL不存在")
		}
		return fmt.Errorf("查询URL失败: %v", err)
	}

	// 验证新的URL（如果提供）
	if originalURL != "" {
		validatedURL, err := s.validateURL(originalURL)
		if err != nil {
			return err
		}
		originalURL = validatedURL
	}

	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}

	if originalURL != "" {
		updates["original_url"] = originalURL
	}

	if title != "" {
		updates["title"] = title
	}

	if expiresAt != nil {
		updates["expires_at"] = expiresAt
	}

	if active != url.IsActive {
		updates["is_active"] = active
	}

	err := s.db.Model(&url).Updates(updates).Error
	if err != nil {
		return err
	}

	// 更新成功后，同步更新缓存
	// 如果URL变为非活跃状态，从缓存中删除
	if !url.IsActive {
		s.cacheManager.DeleteURL(url.ShortCode)
	} else {
		// 重新查询更新后的数据并更新缓存
		var updatedURL models.URL
		if err := s.db.First(&updatedURL, id).Error; err == nil {
			s.cacheManager.SetURL(updatedURL.ShortCode, &updatedURL)
		}
	}

	return nil
}

// DeleteURL 删除URL
func (s *URLService) DeleteURL(id uint, deletedBy string) error {
	var url models.URL
	if err := s.db.First(&url, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("URL不存在")
		}
		return fmt.Errorf("查询URL失败: %v", err)
	}

	// 软删除
	err := s.db.Delete(&url).Error
	if err != nil {
		return err
	}

	// 删除成功后，从缓存中移除
	s.cacheManager.DeleteURL(url.ShortCode)
	return nil
}

// BatchDeleteURLs 批量删除URL
func (s *URLService) BatchDeleteURLs(ids []uint, deletedBy string) error {
	if len(ids) == 0 {
		return errors.New("没有要删除的URL")
	}

	// 先查询要删除的URL的短代码，用于清理缓存
	// 添加权限验证：非admin用户只能删除自己创建的URL
	var urls []models.URL
	query := s.db.Where("id IN ?", ids)
	if deletedBy != "admin" {
		query = query.Where("created_by = ?", deletedBy)
	}
	err := query.Find(&urls).Error
	if err != nil {
		return fmt.Errorf("查询URL失败: %v", err)
	}

	// 检查是否有权限删除所有请求的URL
	if len(urls) != len(ids) {
		return errors.New("部分URL不存在或无权限删除")
	}

	// 获取要删除的URL的ID列表
	urlIds := make([]uint, len(urls))
	for i, u := range urls {
		urlIds[i] = u.ID
	}

	// 批量删除
	err = s.db.Where("id IN ?", urlIds).Delete(&models.URL{}).Error
	if err != nil {
		return err
	}

	// 删除成功后，从缓存中移除所有相关URL
	for _, url := range urls {
		s.cacheManager.DeleteURL(url.ShortCode)
	}

	return nil
}

// ToggleURLStatus 切换URL状态
func (s *URLService) ToggleURLStatus(id uint, updatedBy string) error {
	var url models.URL
	if err := s.db.First(&url, id).Error; err != nil {
		return err
	}

	err := s.db.Model(&url).Updates(map[string]interface{}{
		"is_active":  !url.IsActive,
		"updated_at": time.Now(),
	}).Error
	if err != nil {
		return err
	}

	// 状态切换成功后，同步更新缓存
	if url.IsActive {
		// 原来是活跃的，现在变为非活跃，从缓存中删除
		s.cacheManager.DeleteURL(url.ShortCode)
	} else {
		// 原来是非活跃的，现在变为活跃，重新加载到缓存
		var updatedURL models.URL
		if err := s.db.First(&updatedURL, id).Error; err == nil {
			s.cacheManager.SetURL(updatedURL.ShortCode, &updatedURL)
		}
	}

	return nil
}

// BatchToggleURLs 批量切换URL状态
func (s *URLService) BatchToggleURLs(ids []uint, active bool, username string) error {
	if len(ids) == 0 {
		return errors.New("没有要操作的URL")
	}

	// 先查询要操作的URL，用于缓存同步
	var urls []models.URL
	query := s.db.Where("id IN ?", ids)
	if username != "admin" {
		query = query.Where("created_by = ?", username)
	}
	err := query.Find(&urls).Error
	if err != nil {
		return fmt.Errorf("查询URL失败: %v", err)
	}

	// 检查权限：非管理员只能操作自己的URL
	updateQuery := s.db.Model(&models.URL{}).Where("id IN ?", ids)
	if username != "admin" {
		updateQuery = updateQuery.Where("created_by = ?", username)
	}

	err = updateQuery.Updates(map[string]interface{}{
		"is_active":  active,
		"updated_at": time.Now(),
	}).Error
	if err != nil {
		return err
	}

	// 批量操作成功后，同步更新缓存
	for _, url := range urls {
		if !active {
			// 设置为非活跃，从缓存中删除
			s.cacheManager.DeleteURL(url.ShortCode)
		} else {
			// 设置为活跃，重新加载到缓存
			var updatedURL models.URL
			if err := s.db.First(&updatedURL, url.ID).Error; err == nil {
				s.cacheManager.SetURL(updatedURL.ShortCode, &updatedURL)
			}
		}
	}

	return nil
}

// CleanupExpiredURLs 清理过期的URL
func (s *URLService) CleanupExpiredURLs() error {
	// 先查询要清理的URL，用于缓存同步
	var expiredURLs []models.URL
	err := s.db.Where("expires_at IS NOT NULL AND expires_at < ? AND is_active = ?", time.Now(), true).Find(&expiredURLs).Error
	if err != nil {
		return fmt.Errorf("查询过期URL失败: %v", err)
	}

	// 更新数据库
	err = s.db.Model(&models.URL{}).Where("expires_at IS NOT NULL AND expires_at < ?", time.Now()).Update("is_active", false).Error
	if err != nil {
		return err
	}

	// 清理成功后，从缓存中移除过期的URL
	for _, url := range expiredURLs {
		s.cacheManager.DeleteURL(url.ShortCode)
	}

	return nil
}

// SyncClickCounts 同步点击计数
func (s *URLService) SyncClickCounts() {
	clickCounts := s.cacheManager.GetAllClickCounts()
	for shortCode, count := range clickCounts {
		err := s.db.Model(&models.URL{}).Where("short_code = ?", shortCode).Update("click_count", gorm.Expr("click_count + ?", count)).Error
		if err != nil {
			fmt.Printf("同步点击计数失败 [%s]: %v\n", shortCode, err)
		}
	}
	s.cacheManager.ClearClickCounts()
}

// StartClickCountSync 启动点击计数同步
func (s *URLService) StartClickCountSync() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.SyncClickCounts()
			}
		}
	}()
}

// GetURLByID 根据ID获取URL
func (s *URLService) GetURLByID(id uint) (*models.URL, error) {
	var url models.URL
	err := s.db.First(&url, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("URL不存在")
		}
		return nil, fmt.Errorf("查询URL失败: %v", err)
	}
	return &url, nil
}

// GetExpiredURLs 获取过期的URL
func (s *URLService) GetExpiredURLs() ([]models.URL, error) {
	var urls []models.URL
	err := s.db.Where("expires_at IS NOT NULL AND expires_at < ? AND is_active = ?", time.Now(), true).Find(&urls).Error
	return urls, err
}

// // GetClickStats 获取点击统计
// func (s *URLService) GetClickStats(urlID uint, days int) ([]map[string]interface{}, error) {
// 	return models.GetDailyStats(urlID, days)
// }

// WarmupCache 预热缓存 - 加载所有有效且未过期的短链接到缓存
func (s *URLService) WarmupCache() {
	go func() {
		// 查询所有有效且未过期的短链接
		var urls []models.URL
		err := s.db.Where("is_active = ? AND (expires_at IS NULL OR expires_at > ?)",
			true, time.Now()).Find(&urls).Error
		if err != nil {
			return
		}
		// 将查询到的URL加载到缓存中
		for _, url := range urls {
			s.cacheManager.SetURL(url.ShortCode, &url)
		}
	}()
}

// generateShortCodeFromURL 基于原始URL生成base62短代码
func (s *URLService) generateShortCodeFromURL(originalURL string) string {
	const base62Charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	const length = 6

	// 使用SHA256哈希URL
	hash := sha256.Sum256([]byte(originalURL))

	// 将前8字节转换为uint64
	num := binary.BigEndian.Uint64(hash[:8])

	// 转换为base62
	result := make([]byte, length)
	for i := length - 1; i >= 0; i-- {
		result[i] = base62Charset[num%62]
		num /= 62
	}

	return string(result)
}
