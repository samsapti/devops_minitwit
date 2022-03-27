package main

import (
	"database/sql"
	"fmt"
	"strconv"

	"crypto/md5"
	"encoding/hex"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/bcrypt"

	ctrl "minitwit/controllers"
	mntr "minitwit/monitoring"
)

type SessionData struct {
	Flashes []interface{}
	User    ctrl.User
}

type TimelineData struct {
	RequestUrl   string
	Followed     bool
	Profile_User ctrl.User
	Messages     []map[string]interface{}
	SessionData  SessionData
}

var (
	DB    *sql.DB
	store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))
)

const (
	perPage = 30
	port    = 8080
)

func main() {
	ctrl.Init_db(ctrl.InitDBSchema, ctrl.DBPath)

	r := mux.NewRouter()

	// Load CSS
	r.PathPrefix("/static/css/").Handler(http.StripPrefix("/static/css/", http.FileServer(http.Dir("./static/css/"))))

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

	http.Handle("/", mntr.MiddlewareMetrics(r))

	/*
		Prometheus metrics setup
	*/

	http.Handle("/metrics", promhttp.Handler())

	// Use goroutine because http.ListenAndServe() is a blocking method
	go func() {
		if err := http.ListenAndServe(":2112", nil); err != nil {
			log.Fatal("Error: ", err)
		}
	}()

	/*
		Start app server
	*/

	srv := &http.Server{
		Addr:         "0.0.0.0:" + strconv.Itoa(port),
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}

	DB = ctrl.Connect_db(ctrl.DBPath)
	log.Printf("Starting app on port %d\n", port)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal("Error: ", err)
	}
}

func favicon(w http.ResponseWriter, r *http.Request) {}

func get_user_id(username string) int64 {
	rows, err := DB.Query("SELECT user.user_id FROM user WHERE username = ?", username)
	rv := ctrl.HandleQuery(rows, err)

	if rv != nil || len(rv) != 0 {
		return rv[0]["user_id"].(int64)
	}

	return -1
}

func format_datetime(time time.Time) string {
	return time.UTC().Format("2006-01-02 @ 15:04")
}

func format_datetime2(utc int64) string {
	return time.Unix(utc, 0).Format("2006-01-02 @ 15:04")
}

// Default size: 80
func gravatar_url(email string, size int) string {
	email = strings.TrimSpace(email)
	md := md5.New()
	io.WriteString(md, email)
	return fmt.Sprintf("http://www.gravatar.com/avatar/%s?d=identicon&s=%d", hex.EncodeToString(md.Sum(nil)), size)
}

func getUserSession(w http.ResponseWriter, r *http.Request) (*sessions.Session, ctrl.User) {
	session, _ := store.Get(r, "user-session")

	user := ctrl.User{}

	if session.Values["user_id"] == nil || session.Values["username"] == nil {
		user.Id = -1
		user.Username = ""
		clearUserSessionData(w, r)
	} else {
		user.Id = session.Values["user_id"].(int64)
		user.Username = session.Values["username"].(string)
	}

	log.Println("getUserSession, User:", user)
	return session, user
}

func getMessages(w http.ResponseWriter, r *http.Request, public bool) []map[string]interface{} {
	_, user := getUserSession(w, r)

	if public {
		rows, err := DB.Query("select message.*, user.* from message, user where message.flagged = 0 and message.author_id = user.user_id order by message.pub_date desc limit ?", perPage)
		res := ctrl.HandleQuery(rows, err)
		log.Printf("Showing %d results", len(res))
		return res
	} else {
		rows, err := DB.Query("select message.*, user.* from message, user where message.flagged = 0 and message.author_id = user.user_id and (user.user_id = ? or user.user_id in (select whom_id from follower where who_id = ?)) order by message.pub_date desc limit ?", user.Id, user.Id, perPage)
		res := ctrl.HandleQuery(rows, err)
		log.Printf("Showing %d results", len(res))
		return res
	}
}

func setupTimelineTemplates(data TimelineData) *template.Template {
	tmpl, err := template.New("timeline.html").Funcs(template.FuncMap{
		"gravatar_url": func(email string, size int) string {
			return gravatar_url(email, size)
		},
		"format_datetime": func(time int64) string {
			return format_datetime2(time)
		},
		"timeline_title": func() string {
			if data.RequestUrl == "/public" {
				return "Public Timeline"
			} else if data.RequestUrl[0] == '/' && len(data.RequestUrl) > 1 {
				return data.Profile_User.Username + "'s Timeline"
			} else {
				return "My Timeline"
			}
		},
		"requestUserTimeline": func() bool {
			if data.RequestUrl[0] == '/' && len(data.RequestUrl) > 1 && data.RequestUrl != "/public" {
				return true
			}
			return false
		},
	}).ParseFiles("static/timeline.html", "static/layout.html")

	ctrl.CheckError(err)

	return tmpl
}

func timeline(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	fmt.Println("We got a visitor from: " + r.RemoteAddr)

	_, user := getUserSession(w, r)

	if user.Username == "" {
		http.Redirect(w, r, "/public", http.StatusSeeOther)
		return
	}
	// offset?

	messages := getMessages(w, r, false)

	data := TimelineData{
		RequestUrl:  r.URL.Path,
		Messages:    messages,
		SessionData: SessionData{User: ctrl.User{Username: user.Username}},
	}

	tmpl := setupTimelineTemplates(data)
	tmpl.Execute(w, data)
}

func public_timeline(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	fmt.Println("public timeline!")

	messages := getMessages(w, r, true)
	_, user := getUserSession(w, r)

	data := TimelineData{
		RequestUrl:  r.URL.Path,
		Messages:    messages,
		SessionData: SessionData{User: ctrl.User{Username: user.Username}},
	}

	tmpl := setupTimelineTemplates(data)
	tmpl.Execute(w, data)
}

func user_timeline(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	vars := mux.Vars(r)

	rows, err := DB.Query("select * from user where username = ?", vars["username"])
	profile_user := ctrl.HandleQuery(rows, err)

	if len(profile_user) < 1 {
		w.WriteHeader(404)
		return
	}

	_, user := getUserSession(w, r)
	followed := false

	if user.Id != -1 {
		rows, err := DB.Query("select 1 from follower where follower.who_id = ? and follower.whom_id = ?", user.Id, profile_user[0]["user_id"].(int64))
		res := ctrl.HandleQuery(rows, err)
		followed = res != nil || len(res) != 0
	}

	data := TimelineData{
		RequestUrl:   r.URL.Path,
		Followed:     followed,
		Messages:     getMessages(w, r, true),
		Profile_User: ctrl.User{Username: profile_user[0]["username"].(string)},
		SessionData:  SessionData{User: ctrl.User{Username: user.Username}},
	}

	tmpl := setupTimelineTemplates(data)
	tmpl.Execute(w, data)
}

func follow_user(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)

	session, user := getUserSession(w, r)
	if user.Username == "" {
		w.WriteHeader(401)
		return
	}
	vars := mux.Vars(r)
	whom_id := get_user_id(vars["username"])
	if whom_id == -1 {
		w.WriteHeader(404)
		return
	}
	_, err := DB.Exec("insert into follower (who_id, whom_id) values (?, ?)", user.Id, whom_id)
	ctrl.CheckError(err)
	session.AddFlash("You are now following %s", vars["username"])
	str := "/" + vars["username"]
	http.Redirect(w, r, str, http.StatusSeeOther)
}

func unfollow_user(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)

	session, user := getUserSession(w, r)
	if user.Username == "" {
		w.WriteHeader(401)
		return
	}
	vars := mux.Vars(r)
	whom_id := get_user_id(vars["username"])
	if whom_id == -1 {
		w.WriteHeader(404)
		return
	}
	_, err := DB.Exec("delete from follower where who_id=? and whom_id=?", user.Id, whom_id)
	ctrl.CheckError(err)
	session.AddFlash("You are no longer following %s", vars["username"])
	session.Save(r, w)
	str := "/" + vars["username"]
	http.Redirect(w, r, str, http.StatusSeeOther)
}

func add_message(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	session, user := getUserSession(w, r)

	if user.Id == -1 {
		w.WriteHeader(401)
		return
	}
	if r.FormValue("text") != "" {
		_, err := DB.Exec("insert into message (author_id, text, pub_date, flagged) values (?, ?, ?, 0)", user.Id, r.FormValue("text"), int(time.Now().Unix()))
		ctrl.CheckError(err)
		session.AddFlash("Your message was recorded")
		session.Save(r, w)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func login(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)

	session, user := getUserSession(w, r)
	user_id := user.Id
	username := user.Username
	if user_id != -1 {
		fmt.Println("user_id is", user_id)   // delete later
		fmt.Println("username is", username) // delete later
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	var error string
	if r.Method == "POST" {
		inputUsername := r.FormValue("username")
		inputPassword := r.FormValue("password")

		rows, err := DB.Query("select * from user where username = ?", inputUsername)
		user := ctrl.HandleQuery(rows, err)

		if user == nil {
			error = "Invalid username"
		} else if !check_password_hash(inputPassword, user[0]["pw_hash"].(string)) {
			error = "Invalid password"
		} else {
			session.AddFlash("You were logged in")
			session.Values["user_id"] = user[0]["user_id"]
			session.Values["username"] = user[0]["username"]
			session.Save(r, w)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	tmpl, err := template.ParseFiles("static/login.html", "static/layout.html")
	ctrl.CheckError(err)
	data := struct {
		Error       string
		SessionData SessionData
	}{
		Error:       error,
		SessionData: SessionData{Flashes: session.Flashes()},
	}
	tmpl.Execute(w, data)
}

func register(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	session, _ := store.Get(r, "user-session")
	user_id := session.Values["user_id"]
	if user_id != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	var error string
	if r.Method == "POST" {
		if r.FormValue("username") == "" {
			error = "You have to enter a username"
		} else if r.FormValue("email") == "" || !strings.Contains(r.FormValue("email"), "@") {
			error = "You have to enter a valid email address"
		} else if r.FormValue("password") == "" {
			error = "You have to enter a password"
		} else if r.FormValue("password") != r.FormValue("password2") {
			error = "The two passwords do not match"
		} else if get_user_id(r.FormValue("username")) != -1 {
			error = "The username is already taken"
		} else {
			hashed_pw, err := ctrl.Generate_password_hash(r.FormValue("password"))
			ctrl.CheckError(err)
			_, err = DB.Exec("insert into user (username, email, pw_hash) values (?, ?, ?)", r.FormValue("username"), r.FormValue("email"), hashed_pw)
			ctrl.CheckError(err)
			session.AddFlash("You were successfully registered and can login now")
			session.Save(r, w)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
	}
	tmpl, err := template.ParseFiles("static/register.html", "static/layout.html")
	ctrl.CheckError(err)
	data := struct {
		Error       string
		SessionData SessionData
	}{
		Error:       error,
		SessionData: SessionData{Flashes: session.Flashes()},
	}
	tmpl.Execute(w, data)
}

func logout(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	session, _ := store.Get(r, "user-session")
	session.AddFlash("You were logged out")
	clearUserSessionData(w, r)
	http.Redirect(w, r, "/public", http.StatusSeeOther)
}

func clearUserSessionData(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	delete(session.Values, "user_id")  //session.Values["user_id"] = nil
	delete(session.Values, "username") //session.Values["username"] = nil
	session.Save(r, w)
}

// The function below has been copied from: https://gowebexamples.com/password-hashing/
func check_password_hash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
