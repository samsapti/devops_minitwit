package main

import (
	"C"
	"database/sql"
	"fmt"

	//"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	pongo2 "github.com/flosch/pongo2"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	sqlite3 "github.com/mattn/go-sqlite3"
)

var DATABASE = "../tmp/minitwit.db"
var PER_PAGE = 30
var DEBUG = true
var SECRET_KEY = "development key"
var loggedIn = false
var store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

var sq = sqlite3.ErrAbort

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", index)
	r.PathPrefix("/styles/").Handler(http.StripPrefix("/styles/", http.FileServer(http.Dir("/static/"))))

	//r.HandleFunc("/", timeline)
	r.HandleFunc("/public", public_timeline)
	r.HandleFunc("/{username}", user_timeline)
	r.HandleFunc("/{username}/follow", follow_user)
	r.HandleFunc("/{username}/unfollow", unfollow_user)
	r.HandleFunc("/add_message", add_message)
	r.HandleFunc("/login", login)
	r.HandleFunc("/register", register)
	r.HandleFunc("/logout", logout)
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
func query_db(query string, args []string, one bool) []map[interface{}]interface{} {
	db := connect_db()
	cur, err := db.Query(query, args)
	checkError(err)
	defer cur.Close()
	if one {
		cur.Scan()
	}
	var m []map[interface{}]interface{}
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

func before_request() {

}

func after_request() {

}

func timeline(w http.ResponseWriter, r *http.Request) {
	fmt.Println("We got a visitor from: " + r.RemoteAddr)
	if !loggedIn {
		http.Redirect(w, r, "/public", http.StatusOK)
		return
	}

	/* w.Write([]byte("Hello world!"))
	vars := mux.Vars(r)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Category: %v\n", vars["category"]) */
}

func public_timeline(w http.ResponseWriter, r *http.Request) {

	fmt.Println("public timeline!")
}

func user_timeline(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Category: %v\n", vars["username"])
}

func follow_user(w http.ResponseWriter, r *http.Request) {
	if !loggedIn {
		w.WriteHeader(401)
	}
	vars := mux.Vars(r)
	whom_id := get_user_id(vars["username"])
	if whom_id == 0 {
		w.WriteHeader(404)
	}
	db := connect_db()
	session, _ := store.Get(r, "user-session")
	rv, err := db.Query("insert into follower (who_id, whom_id) values (?, ?)", session.Values["user_id"], whom_id)
	checkError(err)
	defer rv.Close()
	// Add flash message
	str := "/{username}" // TODO: format a string here containing the user_name to be used below
	http.Redirect(w, r, str, http.StatusSeeOther)
}

func unfollow_user(w http.ResponseWriter, r *http.Request) {

}

func add_message(w http.ResponseWriter, r *http.Request) {

}

func login(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	user_id := session.Values["user_id"]
	if user_id != 0 {
		fmt.Println("user_id is", user_id)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if r.Method == "POST" {
		user := query_db("select * from user where username = ?", r.Form["username"], true)
		if user[0] == nil {

		} else if /* check password hash */ 1 == 2 {

		} else {
			// Add flash message
			session.Values["user_id"] = user[0]["user_id"]
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}
	session.Save(r, w)
	// render_template
}

func register(w http.ResponseWriter, r *http.Request) {

}

func logout(w http.ResponseWriter, r *http.Request) {
	// Add flash message
	session, _ := store.Get(r, "user-session")
	session.Values["user_id"] = 0
	http.Redirect(w, r, "/public", http.StatusSeeOther)
}
