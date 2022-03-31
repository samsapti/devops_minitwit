package controllers

import (
	"errors"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
	AuthorID int    `json:"author_id" gorm:"not null"`
	Text     string `json:"text" gorm:"not null"`
	Date     int64  `json:"pub_date" gorm:"column:pub_date"`
	Flagged  uint8  `json:"flagged"`
	Author   User   `gorm:"foreignKey:AuthorID"`
}

const (
	DBPath       = "/tmp/minitwit.db"
	InitDBSchema = "../sql/db_init.sql"
)

func CheckError(err error) bool {
	if err != nil {
		log.Printf("Error: %s\n", err)
	}

	return err != nil
}

func ConnectDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(DBPath), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})

	if err != nil {
		log.Fatalf("ERROR: failed to connect database: %s", err)
	}

	log.Println("Connecting to database...")
	db.AutoMigrate(&User{}, &Follower{}, &Message{})
	log.Println("Database connected")

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

func main() {
	ConnectDB()
}
