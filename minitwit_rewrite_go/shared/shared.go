package shared

import (
	"database/sql"
	"log"
	"reflect"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var DATABASE = "../tmp/minitwit.db"

type User struct {
	Id       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Pw_hash  string `json:"pw_hash"`
}

type Follower struct {
	Follower_id int `json:"follower_id"`
	Followed_id int `json:"followed_id"`
}

type Message struct {
	Message_id int    `json:"message_id"`
	Author_id  int    `json:"author_id"`
	Text       string `json:"text"`
	Pub_date   int    `json:"pub_date"`
	Flagged    int    `json:"flagged"`
}

func CheckError(err error) bool {
	if err != nil {
		log.Printf("Error: %s\n", err)
	}

	return err != nil
}

func Connect_db() *sql.DB {
	db, err := sql.Open("sqlite3", DATABASE)
	CheckError(err)
	return db
}

func Query_db(query string, args []interface{}, one bool) []map[interface{}]interface{} {
	for i := range args {
		if reflect.TypeOf(args[i]).Kind() == reflect.String {
			query = strings.Replace(query, "?", "'"+args[i].(string)+"'", 1)
		} else if reflect.TypeOf(args[i]).Kind() == reflect.Int {
			query = strings.Replace(query, "?", args[i].(string), 1)
		} else {
			log.Printf("ERROR: unsupported argument type: %A\n", reflect.TypeOf(args[i]))
		}
	}

	db := Connect_db()
	rows, err := db.Query(query)

	if CheckError(err) {
		return nil
	} else {
		defer rows.Close()
	}

	cols, err2 := rows.Columns()

	if CheckError(err2) {
		return nil
	}

	values := make([]interface{}, len(cols))

	for i := range cols {
		values[i] = new(sql.RawBytes)
	}

	var m []map[interface{}]interface{}
	log.Printf("---------\nAttempted query: %s\n---------\n", query)

	for rows.Next() {
		err3 := rows.Scan(values...)

		if CheckError(err3) {
			continue
		}
		//for i := range values {
		//    fmt.Println("values[", i, "] =", values[i])
		//}
		// Now you can check each element of vals for nil-ness,
		// and you can use type introspection and type assertions
		// to fetch the column into a typed variable.
	}

	return m
}

// The function below has been copied from: https://gowebexamples.com/password-hashing/
func Generate_password_hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}
