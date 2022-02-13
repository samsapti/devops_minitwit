package main

import (
	"C"
	"database/sql"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"

	"github.com/gorilla/mux"
)

var DATABASE = "minitwit.db"
var PER_PAGE = 30
var DEBUG = true
var SECRET_KEY = "development key"
var sq = sqlite3.ErrAbort

func main() {
	r := mux.NewRouter()
	//r.HandleFunc("/", timeline)
	r.HandleFunc("/", index)
	r.PathPrefix("/styles/").Handler(http.StripPrefix("/styles/", http.FileServer(http.Dir("./static/css/"))))

	//r.PathPrefix("/").HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
	//	http.ServeFile(rw, r, "./static/layout.html")
	//})

	/* r.HandleFunc("/public", public_timeline)
	r.HandleFunc("/{username}", user_timeline)
	r.HandleFunc("/{username}/follow", follow_user)
	r.HandleFunc("/{username}/unfollow", unfollow_user)
	r.HandleFunc("/add_message", add_message)
	r.HandleFunc("/login", login)
	r.HandleFunc("/register", register)
	r.HandleFunc("/logout", logout) */
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

func index(wr http.ResponseWriter, r *http.Request) {
	tmp := template.Must(template.ParseFiles(("./static/layout.html")))
	if err := tmp.ExecuteTemplate(wr, "layout.html", 1); err != nil {
		http.Error(wr, err.Error(), http.StatusInternalServerError)
	}
}

func checkError(err error) {
	if err != nil {
		log.Fatal("Error: ", err)
	}
}

func connect_db() *sql.DB {
	db, err := sql.Open("sqlite3", "DATABASE")
	checkError(err)
	return db
}

func init_db() {
	db := connect_db()
	query, err := ioutil.ReadFile("schema.sql")
	checkError(err)
	db.Exec(string(query))
}

// Fix this function!
func query_db(query string, args []string, one bool) {
	db := connect_db()
	cur, err := db.Query(query, args)
	checkError(err)
	defer cur.Close()
	if one {
		cur.Scan()
	}
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
	w.Write([]byte("Hello world!"))
	vars := mux.Vars(r)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Category: %v\n", vars["category"])
}
