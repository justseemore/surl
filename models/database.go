package models

import (
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDatabase(dbPath string) error {
	var err error

	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})

	if err != nil {
		return err
	}

	// 只迁移URL表，移除ClickStat表
	err = DB.AutoMigrate(&URL{})
	if err != nil {
		return err
	}

	log.Println("Database connected and migrated successfully")
	return nil
}
