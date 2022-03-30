package main

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	ctrl "minitwit/controllers"
)

// gorm.Model is a built-in struct, not used here

type User struct {
	ID       uint   `json:"id"` // Fields named 'ID' are default PK and autoincrement
	Username string `json:"username" gorm:"not null"`
	Email    string `json:"email" gorm:"not null"`
	PwHash   string `json:"pw_hash" gorm:"not null"`
}

type Follower struct {
	FollowerID uint `json:"follower_id" gorm:"primaryKey"`                     // Explicitly declare PK
	FollowedID uint `json:"followed_id" gorm:"primaryKey"`                     // Composite PK
	User       User `gorm:"foreignKey:FollowerID,FollowedID;references:ID,ID"` // FK relationship
}

type Message struct {
	ID       uint   `json:"message_id"`
	AuthorID int    `json:"author_id" gorm:"not null"`
	Text     string `json:"text" gorm:"not null"`
	Date     int64  `json:"pub_date"`
	Flagged  uint8  `json:"flagged"`
}

func main() {
	// Create temporary database in memory
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})

	if err != nil {
		panic("ERROR: failed to connect database")
	}

	// Migrate schema with debugging info
	db.Debug().AutoMigrate(&User{}, &Follower{}, &Message{})

	/*
		USERS TEST
	*/

	pwSals, _ := ctrl.HashPw("sals_secure_passwd")
	pwJkof, _ := ctrl.HashPw("jkof_secure_passwd")

	db.Create(&User{
		Username: "sals",
		Email:    "sals@itu.dk",
		PwHash:   pwSals,
	})

	db.Create(&User{
		Username: "jkof",
		Email:    "jkof@itu.dk",
		PwHash:   pwJkof,
	})

	var sals User
	var jkof User

	db.First(&sals, 1)
	db.First(&jkof, "username = ?", "jkof")

	fmt.Println("sals is " + sals.Username)
	fmt.Println("jkof is " + jkof.Username)
	fmt.Println()

	/*
		FOLLOWER TEST
	*/

	// Make sals follow jkof
	db.Create(&Follower{FollowerID: sals.ID, FollowedID: jkof.ID})

	// Find the created record
	var salsFollowsJkof Follower
	db.First(&salsFollowsJkof, "follower_id = ? AND followed_id = ?", sals.ID, jkof.ID) // inline where clause

	if salsFollowsJkof.FollowerID == 0 {
		fmt.Printf("%s does not follow %s\n", sals.Username, jkof.Username)
	} else {
		fmt.Printf("%s follows %s\n", sals.Username, jkof.Username) // <-- evaluetes to false
	}

	// Deletion
	db.Delete(&salsFollowsJkof)                                                           // Delete the record like this
	db.Where("follower_id = ? AND followed_id = ?", sals.ID, jkof.ID).Delete(&Follower{}) // Or this
	salsFollowsJkof = Follower{}                                                          // Reset salsFollowsJkof
	db.Where(&Follower{FollowerID: sals.ID}).First(&salsFollowsJkof)                      // Retrieve record, this time with a struct

	if salsFollowsJkof.FollowerID == 0 {
		fmt.Printf("%s does not follow %s anymore\n", sals.Username, jkof.Username) // <-- evaluates to true
		fmt.Println()
	} else {
		fmt.Printf("%s still follows %s\n", sals.Username, jkof.Username)
		fmt.Println()
	}

	var deletedUser User
	db.Delete(&deletedUser, "username = ?", "jkof") // Delete jkof

	if deletedUser.ID == 0 {
		fmt.Println("Joachim is dead XO")
	}
}
