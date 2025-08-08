package models

import (
	"time"

	"gorm.io/gorm"
)

// 移除User结构体，改为简单的认证方式

type URL struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	ShortCode    string         `json:"short_code" gorm:"not null;index;uniqueIndex:idx_short_code_deleted"`
	OriginalURL  string         `json:"original_url" gorm:"not null;type:text"`
	Title        string         `json:"title"`
	Description  string         `json:"description" gorm:"type:text"`
	CustomDomain string         `json:"custom_domain"`
	ClickCount   int64          `json:"click_count" gorm:"default:0;index"`
	IsActive     bool           `json:"is_active" gorm:"default:true;index"`
	ExpiresAt    *time.Time     `json:"expires_at" gorm:"index"`
	CreatedBy    string         `json:"created_by" gorm:"not null;index"`
	CreatedAt    time.Time      `json:"created_at" gorm:"index"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:idx_short_code_deleted"`
}

// IsExpired 检查链接是否过期
func (u *URL) IsExpired() bool {
	if u.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*u.ExpiresAt)
}

// GetFullURL 获取完整的短链接URL
func (u *URL) GetFullURL(domain string) string {
	if u.CustomDomain != "" {
		return "https://" + u.CustomDomain + "/" + u.ShortCode
	}
	return "https://" + domain + "/" + u.ShortCode
}

// 移除ClickStat结构体和相关函数
// type ClickStat struct {
//     ID        uint      `json:"id" gorm:"primaryKey"`
//     URLID     uint      `json:"url_id" gorm:"not null;index"`
//     IP        string    `json:"ip" gorm:"index"`
//     UserAgent string    `json:"user_agent" gorm:"type:text"`
//     Referer   string    `json:"referer" gorm:"type:text"`
//     Country   string    `json:"country" gorm:"index"`
//     City      string    `json:"city"`
//     Device    string    `json:"device" gorm:"index"`
//     Browser   string    `json:"browser" gorm:"index"`
//     OS        string    `json:"os" gorm:"index"`
//     ClickedAt time.Time `json:"clicked_at" gorm:"index"`
// }

// 移除GetDailyStats函数
// func GetDailyStats(urlID uint, days int) ([]map[string]interface{}, error) {
//     var stats []map[string]interface{}
//
//     query := `
//         SELECT
//             DATE(clicked_at) as date,
//             COUNT(*) as clicks
//         FROM click_stats
//         WHERE url_id = ? AND clicked_at >= datetime('now', '-' || ? || ' days')
//         GROUP BY DATE(clicked_at)
//         ORDER BY date
//     `
//
//     result := DB.Raw(query, urlID, days).Scan(&stats)
//     return stats, result.Error
// }
