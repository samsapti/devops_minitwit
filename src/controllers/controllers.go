package controllers

import (
	"errors"
	"fmt"
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"golang.org/x/crypto/bcrypt"
)

// TODO: Rename columns when transitioning to PostgreSQL

type User struct {
	ID       uint   `json:"id" gorm:"column:user_id"`
	Username string `json:"username" gorm:"not null"`
	Email    string `json:"email" gorm:"not null"`
	PwHash   string `json:"pw_hash" gorm:"not null"`
}

type Follower struct {
	FollowerID uint `json:"follower_id" gorm:"column:who_id"`
	FollowsID  uint `json:"follows_id" gorm:"column:whom_id"`
	Follower   User `gorm:"foreignKey:FollowerID"`
	Follows    User `gorm:"foreignKey:FollowsID"`
}

type Message struct {
	ID       uint   `json:"message_id" gorm:"column:message_id"`
	AuthorID uint   `json:"author_id" gorm:"not null"`
	Text     string `json:"text" gorm:"not null"`
	Date     int64  `json:"pub_date" gorm:"column:pub_date"`
	Flagged  uint8  `json:"flagged"`
	Author   User   `gorm:"foreignKey:AuthorID"`
}

const (
	DBPath       = "/tmp/minitwit.db"
	InitDBSchema = "../sql/db_init.sql"
)

func ConnectDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(DBPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
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
