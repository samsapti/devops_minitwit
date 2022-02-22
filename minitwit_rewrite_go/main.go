package main

import (
	"C"
	"database/sql"
	"fmt"
	"strconv"

	"crypto/md5"
	"encoding/hex"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	sqlite3 "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var DATABASE = "../tmp/minitwit.db"
var PER_PAGE = 30
var DEBUG = true
var SECRET_KEY = "development key"
var store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

var sq = sqlite3.ErrAbort

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

func main() {
	r := mux.NewRouter()
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	r.HandleFunc("/", timeline)
	r.HandleFunc("/public", public_timeline)
	r.HandleFunc("/add_message", add_message).Methods("POST")
	r.HandleFunc("/login", login).Methods("GET", "POST")
	r.HandleFunc("/register", register).Methods("GET", "POST")
	r.HandleFunc("/logout", logout)
	r.HandleFunc("/favicon.ico", favicon)
	r.HandleFunc("/{username}", user_timeline)
	r.HandleFunc("/{username}/follow", follow_user)
	r.HandleFunc("/{username}/unfollow", unfollow_user)
	http.Handle("/", r)

	srv := &http.Server{
		Handler: r,
		Addr:    "0.0.0.0:8000",
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal("Error: ", err)
	}
}

func favicon(w http.ResponseWriter, r *http.Request) {

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

func init_db() {
	db := Connect_db()
	query, err := ioutil.ReadFile("../schema.sql")

	if CheckError(err) {
		return
	}

	db.Exec(string(query))
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

func get_user_id(username string) int {
	db := Connect_db()
	rv, err := db.Query("select user_id from user where username = ?", username)
	CheckError(err)
	defer rv.Close()
	var userid int
	for rv.Next() {
		err := rv.Scan(&userid)
		CheckError(err)
	}
	return userid
}

func format_datetime(time time.Time) string {
	return time.UTC().Format("2006-01-02 @ 15:04")
}

// Default size: 80
func gravatar_url(email string, size int) string {
	email = strings.TrimSpace(email)
	md := md5.New()
	io.WriteString(md, email)
	return fmt.Sprintf("http://www.gravatar.com/avatar/%s?d=identicon&s=%d", hex.EncodeToString(md.Sum(nil)), size)
}

func before_request() {

}

func after_request() {

}

func timeline(w http.ResponseWriter, r *http.Request) {
	fmt.Println("We got a visitor from: " + r.RemoteAddr)
	session, _ := store.Get(r, "user-session")
	if session.Values["user_id"] == nil {
		http.Redirect(w, r, "/public", http.StatusOK)
		return
	}
	// offset?
	template, err := template.ParseFiles("static/timeline.html")
	CheckError(err)
	messages := Query_db("select message.*, user.* from message, user where message.flagged = 0 and message.author_id = user.user_id and (user.user_id = ? or user.user_id in (select whom_id from follower where who_id = ?)) order by message.pub_date desc limit ?", []interface{}{ /*session.Values["user_id"].(string), session.Values["user_id"].(string)*/ "", "", strconv.Itoa(PER_PAGE)}, false)
	m := map[string]interface{}{
		"messages": messages,
	}
	template.Execute(w, m)
}

func public_timeline(w http.ResponseWriter, r *http.Request) {
	fmt.Println("public timeline!")
	template, err := template.ParseFiles("static/timeline.html")
	CheckError(err)
	messages := Query_db("select message.*, user.* from message, user where message.flagged = 0 and message.author_id = user.user_id order by message.pub_date desc limit ?", []interface{}{strconv.Itoa(PER_PAGE)}, false)
	m := map[string]interface{}{
		"messages": messages,
	}
	template.Execute(w, m)
}

func user_timeline(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	profile_user := Query_db("select * from user where username = ?", []interface{}{vars["username"]}, true)

	if len(profile_user) < 1 {
		w.WriteHeader(404)
	}

	followed := false
	session, _ := store.Get(r, "user-session")

	if session.Values["user_id"] != nil {
		ql := []interface{}{"", "" /*, session.Values["user_id"].(string), profile_user[0]["user_id"].(string)*/}
		followed = Query_db("select 1 from follower where follower.who_id = ? and follower.whom_id = ?", ql, true) != nil
	}

	template, err := template.ParseFiles("static/timeline.html")
	CheckError(err)

	m := map[string]interface{}{
		"followed":     followed,
		"profile_user": profile_user,
	}

	template.Execute(w, m)
}

func follow_user(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	if session.Values["user_id"] == nil {
		w.WriteHeader(401)
	}
	vars := mux.Vars(r)
	whom_id := get_user_id(vars["username"])
	if whom_id == 0 {
		w.WriteHeader(404)
	}
	db := Connect_db()
	rv, err := db.Query("insert into follower (who_id, whom_id) values (?, ?)", session.Values["user_id"], whom_id)
	CheckError(err)
	defer rv.Close()
	session.AddFlash("You are now following %s", vars["username"])
	str := "/" + vars["username"]
	http.Redirect(w, r, str, http.StatusSeeOther)
}

func unfollow_user(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	if session.Values["user_id"] == nil {
		w.WriteHeader(401)
	}
	vars := mux.Vars(r)
	whom_id := get_user_id(vars["username"])
	if whom_id == 0 {
		w.WriteHeader(404)
	}
	db := Connect_db()
	rv, err := db.Query("delete from follower where who_id=? and whom_id=?", session.Values["user_id"], whom_id)
	CheckError(err)
	defer rv.Close()
	session.AddFlash("You are no longer following %s", vars["username"])
	str := "/" + vars["username"]
	http.Redirect(w, r, str, http.StatusSeeOther)
}

func add_message(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	if session.Values["user_id"] == nil {
		w.WriteHeader(401)
	}
	if r.Form["text"] != nil {
		db := Connect_db()
		rv, err := db.Query("insert into message (author_id, text, pub_date, flagged) values (?, ?, ?, 0)", session.Values["user_id"], "" /*r.Form["text"]*/, time.Now())
		CheckError(err)
		defer rv.Close()
		session.AddFlash("Your message was recorded")
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func login(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	user_id := session.Values["user_id"]
	if user_id != nil {
		fmt.Println("user_id is", user_id) // delete later
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	var error string
	if r.Method == "POST" {
		user := Query_db("select * from user where username = ?", []interface{}{""} /*r.Form["username"]*/, true)
		if user[0] == nil {
			error = "Invalid username"
		} else if check_password_hash(r.Form["password"][0], user[0]["pw_hash"].(string)) {
			error = "Invalid password"
		} else {
			session.AddFlash("You were logged in")
			session.Values["user_id"] = user[0]["user_id"]
			session.Save(r, w)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}
	template, err := template.ParseFiles("static/login.html")
	CheckError(err)
	m := map[string]interface{}{
		"error": error,
	}
	template.Execute(w, m)
}

func register(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	user_id := session.Values["user_id"]
	if user_id != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	var error string
	if r.Method == "POST" {
		if r.Form["username"] == nil {
			error = "You have to enter a username"
		} else if r.Form["email"] == nil /*|| !r.Form["email"].contains("@")*/ {
			error = "You have to enter a valid email address"
		} else if r.Form["password"] == nil {
			error = "You have to enter a password"
		} else if r.Form["password"] != nil /*r.Form["password2"] */ {
			error = "The two passwords do not match"
		} else if get_user_id(r.Form["username"][0]) != 0 {
			error = "The username is already taken"
		} else {
			db := Connect_db()
			hashed_pw, err := Generate_password_hash(r.Form["password"][0])
			CheckError(err)
			rv, err := db.Query("insert into user (username, email, pw_hash) values (?, ?, ?)", r.Form["username"], r.Form["email"], hashed_pw)
			CheckError(err)
			defer rv.Close()
			session.AddFlash("You were successfully registered and can login now")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
	}
	template, err := template.ParseFiles("static/register.html")
	CheckError(err)
	m := map[string]interface{}{
		"error": error,
	}
	template.Execute(w, m)
}

func logout(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	session.AddFlash("You were logged in")
	session.Values["user_id"] = nil
	http.Redirect(w, r, "/public", http.StatusSeeOther)
}

// The two functions below have been copied from: https://gowebexamples.com/password-hashing/
func Generate_password_hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}
func check_password_hash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
