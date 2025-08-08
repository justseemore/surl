package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/justseemore/surl/models"
	"github.com/patrickmn/go-cache"
)

type Manager struct {
	// 一级缓存：内存缓存
	memCache *cache.Cache
	// 二级缓存：Redis
	redisClient *redis.Client
	ctx         context.Context
	expiry      time.Duration
	useRedis    bool // 标识是否使用Redis

	// 内存点击计数存储
	memClickCounts map[string]int64
	memClickMutex  sync.RWMutex
}

func NewCacheManager(redisAddr string, redisPassword string, redisDB int, cacheExpiry int) *Manager {
	// 内存缓存
	memCache := cache.New(time.Duration(cacheExpiry)*time.Minute, 10*time.Minute)

	manager := &Manager{
		memCache:       memCache,
		ctx:            context.Background(),
		expiry:         time.Duration(cacheExpiry) * time.Minute,
		useRedis:       false,
		memClickCounts: make(map[string]int64),
	}

	// 如果提供了Redis地址，尝试连接Redis
	if redisAddr != "" {
		// 配置Redis连接选项
		options := &redis.Options{
			Addr: redisAddr,
			DB:   redisDB,
		}

		// 如果提供了密码，设置密码
		if redisPassword != "" {
			options.Password = redisPassword
		}

		rdb := redis.NewClient(options)

		// 测试Redis连接
		if err := rdb.Ping(manager.ctx).Err(); err != nil {
			log.Printf("Redis连接失败，仅使用内存缓存: %v", err)
		} else {
			manager.redisClient = rdb
			manager.useRedis = true
			log.Println("Redis缓存已启用")
		}
	}

	return manager
}

// NewManager 新的构造函数，用于兼容已有代码
func NewManager(redisAddr string, redisPassword string, redisDB int, cacheExpiry int) *Manager {
	// 使用默认参数，仅使用内存缓存
	return NewCacheManager(redisAddr, redisPassword, redisDB, cacheExpiry)
}

// Close 关闭缓存管理器
func (c *Manager) Close() error {
	if c.useRedis && c.redisClient != nil {
		return c.redisClient.Close()
	}
	return nil
}

// GetURL 获取URL
func (c *Manager) GetURL(shortCode string) (*models.URL, bool) {
	key := fmt.Sprintf("url:%s", shortCode)

	// 1. 先查内存缓存
	if data, found := c.memCache.Get(key); found {
		if url, ok := data.(*models.URL); ok {
			return url, true
		}
	}

	// 2. 查Redis（如果可用）
	if c.useRedis {
		val, err := c.redisClient.Get(c.ctx, key).Result()
		if err == nil {
			var url models.URL
			if err := json.Unmarshal([]byte(val), &url); err == nil {
				// 存入内存缓存
				c.memCache.Set(key, &url, c.expiry)
				return &url, true
			}
		}
	}

	return nil, false
}

// SetURL 设置URL缓存
func (c *Manager) SetURL(shortCode string, url *models.URL) {
	key := fmt.Sprintf("url:%s", shortCode)

	// 存入内存缓存
	c.memCache.Set(key, url, c.expiry)

	// 存入Redis（如果可用）
	if c.useRedis {
		if data, err := json.Marshal(url); err == nil {
			if err := c.redisClient.Set(c.ctx, key, data, c.expiry).Err(); err != nil {
				log.Printf("Redis设置缓存失败: %v", err)
			}
		}
	}
}

// DeleteURL 删除缓存
func (c *Manager) DeleteURL(shortCode string) {
	key := fmt.Sprintf("url:%s", shortCode)
	c.memCache.Delete(key)

	if c.useRedis {
		if err := c.redisClient.Del(c.ctx, key).Err(); err != nil {
			log.Printf("Redis删除缓存失败: %v", err)
		}
	}
}

// IncrementClick 增加点击计数（异步）
func (c *Manager) IncrementClick(shortCode string) {
	go func() {
		// 优先使用Redis，如果Redis不可用则使用内存计数
		if c.useRedis {
			key := fmt.Sprintf("clicks:%s", shortCode)
			if err := c.redisClient.Incr(c.ctx, key).Err(); err != nil {
				log.Printf("Redis增加点击计数失败: %v", err)
				// Redis失败时使用内存计数
				c.incrementMemoryClick(shortCode)
				return
			}
			if err := c.redisClient.Expire(c.ctx, key, 24*time.Hour).Err(); err != nil {
				log.Printf("Redis设置过期时间失败: %v", err)
			}
		} else {
			// 使用内存计数
			c.incrementMemoryClick(shortCode)
		}
	}()
}

// incrementMemoryClick 内存点击计数增加
func (c *Manager) incrementMemoryClick(shortCode string) {
	c.memClickMutex.Lock()
	defer c.memClickMutex.Unlock()
	c.memClickCounts[shortCode]++
}

// GetAndResetClicks 获取并重置点击计数
func (c *Manager) GetAndResetClicks(shortCode string) int64 {
	var count int64

	// 先尝试从Redis获取
	if c.useRedis {
		key := fmt.Sprintf("clicks:%s", shortCode)
		val, err := c.redisClient.GetDel(c.ctx, key).Result()
		if err == nil {
			redisCount, err := strconv.ParseInt(val, 10, 64)
			if err == nil {
				count += redisCount
			}
		}
	}

	// 获取并重置内存计数
	c.memClickMutex.Lock()
	memCount := c.memClickCounts[shortCode]
	delete(c.memClickCounts, shortCode)
	c.memClickMutex.Unlock()

	count += memCount
	return count
}

// IncrementClickCount 增加点击计数（兼容旧接口）
func (c *Manager) IncrementClickCount(shortCode string) {
	c.IncrementClick(shortCode)
}

// GetAllClickCounts 获取所有点击计数
func (c *Manager) GetAllClickCounts() map[string]int64 {
	results := make(map[string]int64)

	// 获取Redis中的计数
	if c.useRedis {
		// 使用 Redis 的 SCAN 命令获取所有 clicks:* 键
		iter := c.redisClient.Scan(c.ctx, 0, "clicks:*", 0).Iterator()
		for iter.Next(c.ctx) {
			key := iter.Val()
			val, err := c.redisClient.Get(c.ctx, key).Result()
			if err == nil {
				count, err := strconv.ParseInt(val, 10, 64)
				if err == nil {
					shortCode := strings.TrimPrefix(key, "clicks:")
					results[shortCode] = count
				}
			}
		}

		if err := iter.Err(); err != nil {
			log.Printf("Redis扫描失败: %v", err)
		}
	}

	// 合并内存中的计数
	c.memClickMutex.RLock()
	for shortCode, count := range c.memClickCounts {
		results[shortCode] += count
	}
	c.memClickMutex.RUnlock()

	return results
}

// ClearClickCounts 清空所有点击计数
func (c *Manager) ClearClickCounts() {
	// 清空Redis中的计数
	if c.useRedis {
		// 获取所有 clicks:* 键并删除
		iter := c.redisClient.Scan(c.ctx, 0, "clicks:*", 0).Iterator()
		keys := []string{}
		for iter.Next(c.ctx) {
			keys = append(keys, iter.Val())
		}

		if len(keys) > 0 {
			if err := c.redisClient.Del(c.ctx, keys...).Err(); err != nil {
				log.Printf("Redis批量删除失败: %v", err)
			}
		}

		if err := iter.Err(); err != nil {
			log.Printf("Redis扫描失败: %v", err)
		}
	}

	// 清空内存中的计数
	c.memClickMutex.Lock()
	c.memClickCounts = make(map[string]int64)
	c.memClickMutex.Unlock()
}
