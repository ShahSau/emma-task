package common

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init() *gorm.DB {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	var db *gorm.DB
	var err error

	// RETRY LOOP: Try to connect for 30 seconds
	for i := 1; i <= 15; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			log.Println("Database connection established!")
			break
		}

		log.Printf("Failed to connect to database (Attempt %d/15). Retrying in 2s... Error: %v", i, err)
		time.Sleep(2 * time.Second)
	}

	// If still failing after 30 seconds, then crash
	if err != nil {
		log.Fatal("Could not connect to database after retries: ", err)
	}

	DB = db
	return DB
}

func GetDB() *gorm.DB {
	return DB
}
