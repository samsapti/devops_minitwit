package controllers

import (
	"database/sql"
	"errors"
	"log"
	"reflect"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       uint   `json:"id"`
	Username string `json:"username" gorm:"not null"`
	Email    string `json:"email" gorm:"not null"`
	PwHash   string `json:"pw_hash" gorm:"not null"`
}

type Follower struct {
	FollowerID uint `json:"follower_id" gorm:"primaryKey"`
	FollowedID uint `json:"followed_id" gorm:"primaryKey"`
	User       User `gorm:"foreignKey:FollowerID,FollowedID;references:ID,ID"`
}

type Message struct {
	ID       uint   `json:"message_id"`
	AuthorID uint   `json:"author_id" gorm:"not null"`
	Text     string `json:"text" gorm:"not null"`
	Date     int64  `json:"pub_date"`
	Flagged  uint8  `json:"flagged"`
	User     User   `gorm:"foreignKey:AuthorID;references:ID"`
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

func ConnectDB(dbPath string) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})

	if err != nil {
		panic("ERROR: failed to connect database")
	}

	db.AutoMigrate(&Follower{}, &Message{}, &User{})
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

func HandleQuery(rows *sql.Rows, err error) []map[string]interface{} {
	if CheckError(err) {
		return nil
	} else {
		defer rows.Close()
	}

	cols, err := rows.Columns()

	if CheckError(err) {
		return nil
	}

	values := make([]interface{}, len(cols))

	for i := range cols {
		values[i] = new(interface{})
	}

	dicts := make([]map[string]interface{}, 0)
	rowsCount := 0

	for rows.Next() {
		rowsCount++
		err = rows.Scan(values...)
		if CheckError(err) {
			continue
		}

		m := make(map[string]interface{})

		for i, v := range values {
			val := reflect.Indirect(reflect.ValueOf(v)).Interface()
			m[cols[i]] = val
		}

		dicts = append(dicts, m)
	}

	log.Printf("	Columns %v returned dictionaries: %v", cols, dicts)

	log.Printf("Length of dicts: %d", len(dicts))
	return dicts
}

// The function below has been borrowed from: https://gowebexamples.com/password-hashing/
func HashPw(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 8)
	return string(bytes), err
}
