package controllers

import (
	"errors"
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       uint   `json:"id"`
	Username string `json:"username" gorm:"not null"`
	Email    string `json:"email" gorm:"not null"`
	PwHash   string `json:"pw_hash" gorm:"not null"`
}

type Follower struct {
	FollowerID uint `json:"follower_id"`
	FollowsID  uint `json:"follows_id"`
	Follower   User `gorm:"foreignKey:FollowerID"`
	Follows    User `gorm:"foreignKey:FollowsID"`
}

type Message struct {
	ID       uint   `json:"message_id"`
	AuthorID uint   `json:"author_id" gorm:"not null"`
	Text     string `json:"text" gorm:"not null"`
	Date     int64  `json:"pub_date"`
	Flagged  uint8  `json:"flagged"`
	Author   User   `gorm:"foreignKey:AuthorID"`
}

const (
	DBPath       = "/tmp/minitwit.db"
	InitDBSchema = "../sql/db_init.sql"
)

func ConnectDB() *gorm.DB {
	dsn := "host=postgres user=minitwit_user password=" + os.Getenv("DB_PASSWD") + " dbname=minitwit_db port=5432"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "ConnectDB: Error connecting to database: %s\n", err)
		os.Exit(1)
	}

	db.AutoMigrate(&User{}, &Follower{}, &Message{})

	return db
}

func GetUserID(username string, db *gorm.DB) uint {
	var user User
	result := db.First(&user, "username = ?", username)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return 0
	}

	return user.ID
}

// The function below has been borrowed from: https://gowebexamples.com/password-hashing/
func HashPw(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 8)
	return string(bytes), err
}
