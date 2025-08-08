package models

import (
	"log"
	"os"
	"syscall"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDatabase(dbPath string) error {
	lockFile := dbPath + ".lock"

	// 创建锁文件
	file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer file.Close()
	defer os.Remove(lockFile)

	// 尝试获取文件锁
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
	if err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	// 检查数据库是否已存在
	if _, err := os.Stat(dbPath); err == nil {
		// 数据库已存在，直接连接
		return connectToExistingDB(dbPath)
	}

	// 数据库不存在，创建并初始化
	return createAndInitDB(dbPath)
}

func connectToExistingDB(dbPath string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return err
	}
	log.Println("Connected to existing database")
	return nil
}

func createAndInitDB(dbPath string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return err
	}

	// 执行迁移
	err = DB.AutoMigrate(&URL{})
	if err != nil {
		return err
	}

	log.Println("Database created and migrated successfully")
	return nil
}
