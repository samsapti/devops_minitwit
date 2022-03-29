package main

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	ctrl "minitwit/controllers"
)

// gorm.Model is a built-in struct, not used here

type User struct {
	ID       uint   `json:"id"` // Fields named 'ID' are default PK
	Username string `json:"username"`
	Email    string `json:"email"`
	PwHash   string `json:"pw_hash"`
}

type Follow struct {
	WhoID  uint `json:"follower_id" gorm:"primary_key"` // Explicitly declare PK
	WhomID uint `json:"followed_id" gorm:"primary_key"` // Explicitly declare PK
}

type Message struct {
	ID      uint   `json:"message_id"` // Fields named 'ID' are default PK
	UserID  int64  `json:"author_id"`
	Text    string `json:"text"`
	Date    int64  `json:"pub_date"`
	Flagged int64  `json:"flagged"`
}

func main() {
	// Create temporary database in memory
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})

	if err != nil {
		panic("ERROR: failed to connect database")
	}

	db.Debug()

	// Migrate schema
	db.AutoMigrate(&User{}, &Follow{}, &Message{})

	/*
		USERS TEST
	*/

	pwSals, _ := ctrl.Generate_password_hash("sals_secure_passwd")
	pwJkof, _ := ctrl.Generate_password_hash("jkof_secure_passwd")

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
	db.Create(&Follow{WhoID: sals.ID, WhomID: jkof.ID})

	// Find the created record
	var salsFollowsJkof Follow
	db.First(&salsFollowsJkof, "who_id = ? AND whom_id = ?", sals.ID, jkof.ID) // inline where clause

	if salsFollowsJkof.WhoID == 0 {
		fmt.Printf("%s does not follow %s\n", sals.Username, jkof.Username)
	} else {
		fmt.Printf("%s follows %s\n", sals.Username, jkof.Username) // <-- evaluetes to false
	}

	// Deletion
	db.Delete(&salsFollowsJkof)                                          // Delete the record like this
	db.Delete(&Follow{}, "who_id = ? AND whom_id = ?", sals.ID, jkof.ID) // Or this
	db.Delete(&Follow{WhoID: sals.ID, WhomID: jkof.ID})                  // Or this
	salsFollowsJkof = Follow{}                                           // Reset salsFollowsJkof
	db.Where(&Follow{WhoID: sals.ID}).First(&salsFollowsJkof)            // Retrieve record, this time with a struct

	if salsFollowsJkof.WhoID == 0 {
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
