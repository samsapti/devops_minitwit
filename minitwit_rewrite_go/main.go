package main

import (
	"C"
	"database/sql"
	"fmt"
	"strconv"

	"crypto/md5"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	pongo2 "github.com/flosch/pongo2"
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

func main() {
	r := mux.NewRouter()
	//r.HandleFunc("/", index)
	r.PathPrefix("/styles/").Handler(http.StripPrefix("/styles/", http.FileServer(http.Dir("/static/"))))

	r.HandleFunc("/", timeline)
	r.HandleFunc("/public", public_timeline)
	r.HandleFunc("/add_message", add_message).Methods("POST")
	r.HandleFunc("/login", login).Methods("GET", "POST")
	r.HandleFunc("/register", register).Methods("GET", "POST")
	r.HandleFunc("/logout", logout)
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

func index(w http.ResponseWriter, r *http.Request) {
	tmp := pongo2.Must(pongo2.FromFile("./static/layout.html"))
	if err := tmp.ExecuteWriter(pongo2.Context{"query": r.FormValue("query")}, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func checkError(err error) {
	if err != nil {
		log.Fatal("Error: ", err)
	}
}

func connect_db() *sql.DB {
	db, err := sql.Open("sqlite3", DATABASE)
	checkError(err)
	return db
}

func init_db() {
	db := connect_db()
	query, err := ioutil.ReadFile("../schema.sql")
	checkError(err)
	db.Exec(string(query))
}

// Fix this function!
func query_db(query string, args []interface{}, one bool) []map[interface{}]interface{} {
	for i := range args {
		if reflect.TypeOf(args[i]).Kind() == reflect.String {
			query = strings.Replace(query, "?", "'"+args[i].(string)+"'", 1)
		} else if reflect.TypeOf(args[i]).Kind() == reflect.Int {
			query = strings.Replace(query, "?", args[i].(string), 1)
		} else {
			log.Fatalln("ERROR: unsupported argument type:", reflect.TypeOf(args[i]))
		}
	}
	db := connect_db()
	fmt.Printf("Attempted query: %s\n", query)
	cur, err := db.Query(query)
	checkError(err)
	defer cur.Close()
	if one {
		cur.Scan()
	}
	var m []map[interface{}]interface{}
	return m
}

func query_db2(query string, args []interface{}, one bool) []map[interface{}]interface{} {
	for i := range args {
		if reflect.TypeOf(args[i]).Kind() == reflect.String {
			query = strings.Replace(query, "?", "'"+args[i].(string)+"'", 1)
		} else if reflect.TypeOf(args[i]).Kind() == reflect.Int {
			query = strings.Replace(query, "?", args[i].(string), 1)
		} else {
			log.Fatalln("ERROR: unsupported argument type:", reflect.TypeOf(args[i]))
		}
	}
	db := connect_db()
	rows, err := db.Query(query)
	checkError(err)
	defer rows.Close()
	cols, err2 := rows.Columns()
	checkError(err2)
	values := make([]interface{}, len(cols))
	for i := range cols {

		values[i] = new(sql.RawBytes)
	}
	var m []map[interface{}]interface{}
	fmt.Printf("Attempted query: %s\n", query)
	for rows.Next() {
		err = rows.Scan(values...)
		for i := range values {
			fmt.Println("values[", i, "] =", values[i])
		}
		// Now you can check each element of vals for nil-ness,
		// and you can use type introspection and type assertions
		// to fetch the column into a typed variable.
	}
	return m
}

func get_user_id(username string) int {
	db := connect_db()
	rv, err := db.Query("select user_id from user where username = ?", username)
	checkError(err)
	defer rv.Close()
	var userid int
	for rv.Next() {
		err := rv.Scan(&userid)
		checkError(err)
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
	io.WriteString(md, "email")
	return fmt.Sprintf("http://www.gravatar.com/avatar/%s?d=identicon&s=%d", string(md.Sum(nil)), size)
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
	checkError(err)
	messages := query_db2("select message.*, user.* from message, user where message.flagged = 0 and message.author_id = user.user_id and (user.user_id = ? or user.user_id in (select whom_id from follower where who_id = ?)) order by message.pub_date desc limit ?", []interface{}{ /*session.Values["user_id"].(string), session.Values["user_id"].(string)*/ "", "", strconv.Itoa(PER_PAGE)}, false)
	m := map[string]interface{}{
		"messages": messages,
	}
	template.Execute(w, m)
}

func public_timeline(w http.ResponseWriter, r *http.Request) {
	fmt.Println("public timeline!")
	template, err := template.ParseFiles("static/timeline.html")
	checkError(err)
	messages := query_db2("select message.*, user.* from message, user where message.flagged = 0 and message.author_id = user.user_id order by message.pub_date desc limit ?", []interface{}{strconv.Itoa(PER_PAGE)}, false)
	m := map[string]interface{}{
		"messages": messages,
	}
	template.Execute(w, m)
}

func user_timeline(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	profile_user := query_db2("select * from user where username = ?", []interface{}{vars["username"]}, true)
	if len(profile_user) < 1 {
		w.WriteHeader(404)
	}
	followed := false
	session, _ := store.Get(r, "user-session")
	if session.Values["user_id"] != nil {
		ql := []interface{}{"", "" /*, session.Values["user_id"].(string), profile_user[0]["user_id"].(string)*/}
		followed = query_db2("select 1 from follower where follower.who_id = ? and follower.whom_id = ?", ql, true) != nil
	}
	template, err := template.ParseFiles("static/timeline.html")
	checkError(err)
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
	db := connect_db()
	rv, err := db.Query("insert into follower (who_id, whom_id) values (?, ?)", session.Values["user_id"], whom_id)
	checkError(err)
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
	db := connect_db()
	rv, err := db.Query("delete from follower where who_id=? and whom_id=?", session.Values["user_id"], whom_id)
	checkError(err)
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
		db := connect_db()
		rv, err := db.Query("insert into message (author_id, text, pub_date, flagged) values (?, ?, ?, 0)", session.Values["user_id"], "" /*r.Form["text"]*/, time.Now())
		checkError(err)
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
		user := query_db2("select * from user where username = ?", []interface{}{""} /*r.Form["username"]*/, true)
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
	checkError(err)
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
			db := connect_db()
			hashed_pw, err := generate_password_hash(r.Form["password"][0])
			checkError(err)
			rv, err := db.Query("insert into user (username, email, pw_hash) values (?, ?, ?)", r.Form["username"], r.Form["email"], hashed_pw)
			checkError(err)
			defer rv.Close()
			session.AddFlash("You were successfully registered and can login now")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
	}
	template, err := template.ParseFiles("static/register.html")
	checkError(err)
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
func generate_password_hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}
func check_password_hash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
