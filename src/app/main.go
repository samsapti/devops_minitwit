package main

import (
	"errors"
	"fmt"
	"strconv"

	"crypto/sha512"
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
	"gorm.io/gorm"

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
	Messages     []ctrl.Message
	SessionData  SessionData
}

var (
	db    *gorm.DB
	store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))
)

const (
	perPage = 30
	port    = 8080
)

func main() {
	db = ctrl.ConnectDB()
	r := mux.NewRouter()

	// Endpoints
	r.HandleFunc("/", timeline)
	r.HandleFunc("/public", publicTimeline)
	r.HandleFunc("/add_message", addMessage).Methods("POST")
	r.HandleFunc("/login", login).Methods("GET", "POST")
	r.HandleFunc("/register", register).Methods("GET", "POST")
	r.HandleFunc("/logout", logout)
	r.HandleFunc("/{username}", userTimeline)
	r.HandleFunc("/{username}/follow", follow)
	r.HandleFunc("/{username}/unfollow", unfollow)
	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})

	// Load CSS
	r.PathPrefix("/static/css/").Handler(http.StripPrefix("/static/css/", http.FileServer(http.Dir("./static/css/"))))

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

	// Register r as HTTP handler
	http.Handle("/", mntr.MiddlewareMetrics(r, false))

	srv := &http.Server{
		Addr:         "0.0.0.0:" + strconv.Itoa(port),
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}

	log.Printf("Starting app on port %d\n", port)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal("Error: ", err)
	}
}

// Default size: 80
func gravatarUrl(email string, size int) string {
	email = strings.TrimSpace(email)
	hash := sha512.New()
	io.WriteString(hash, email)
	return fmt.Sprintf("https://www.gravatar.com/avatar/%s?d=identicon&s=%d", hex.EncodeToString(hash.Sum(nil)), size)
}

func getUserSession(w http.ResponseWriter, r *http.Request) (*sessions.Session, ctrl.User) {
	session, _ := store.Get(r, "user-session")

	var user ctrl.User

	if session.Values["user_id"] == nil || session.Values["username"] == nil {
		user = ctrl.User{
			ID:       0,
			Username: "",
		}

		clearUserSessionData(w, r)
	} else {
		user = ctrl.User{
			ID:       session.Values["user_id"].(uint),
			Username: session.Values["username"].(string),
		}
	}

	log.Println("getUserSession, User:", user.Username)
	return session, user
}

func getMessages(w http.ResponseWriter, r *http.Request, public bool) ([]ctrl.Message, error) {
	_, user := getUserSession(w, r)
	var messages []ctrl.Message

	if public {
		query := db.Limit(perPage).
			Joins("JOIN user ON message.author_id = user.user_id").
			Order("message.pub_date desc").
			Find("flagged = ?", 0, &messages)

		if query.Error != nil && !errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return nil, query.Error
		}
	} else {
		subquery := db.Select("whom_id").Find("who_id = ?", user.ID, &ctrl.Follower{})
		query := db.Limit(perPage).
			Joins("JOIN user ON message.author_id = user.user_id").
			Order("message.pub_date desc").
			Where("user.user_id = ?", user.ID).
			Or("user.user_id IN ?", subquery).
			Find("flagged = ?", 0, &messages)

		if subquery.Error != nil && !errors.Is(subquery.Error, gorm.ErrRecordNotFound) {
			return nil, subquery.Error
		} else if query.Error != nil && !errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return nil, query.Error
		}
	}

	log.Printf("Showing %d results", len(messages))
	return messages, nil
}

func setupTimelineTemplates(data TimelineData) *template.Template {
	tmpl, err := template.New("timeline.html").Funcs(template.FuncMap{
		"gravatar_url": func(email string, size int) string {
			return gravatarUrl(email, size)
		},
		"format_datetime": func(t int64) string {
			return time.Unix(t, 0).Format("2006-01-02 @ 15:04")
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

	var tmpl *template.Template

	_, user := getUserSession(w, r)

	if user.Username == "" {
		http.Redirect(w, r, "/public", http.StatusSeeOther)
		return
	}
	// offset?

	messages, err := getMessages(w, r, false)

	data := TimelineData{
		RequestUrl:  r.URL.Path,
		Messages:    messages,
		SessionData: SessionData{User: ctrl.User{Username: user.Username}},
	}

	if err != nil {
		w.WriteHeader(500)
		return
	}

	tmpl = setupTimelineTemplates(data)
	tmpl.Execute(w, data)
}

func publicTimeline(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	fmt.Println("public timeline!")

	messages, err := getMessages(w, r, true)

	if err != nil {
		w.WriteHeader(500)
		return
	}

	_, user := getUserSession(w, r)

	data := TimelineData{
		RequestUrl:  r.URL.Path,
		Messages:    messages,
		SessionData: SessionData{User: ctrl.User{Username: user.Username}},
	}

	tmpl := setupTimelineTemplates(data)
	tmpl.Execute(w, data)
}

func userTimeline(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	vars := mux.Vars(r)

	rows, err := db.Query("select * from user where username = ?", vars["username"])
	profile_user := ctrl.HandleQuery(rows, err)

	if len(profile_user) < 1 {
		w.WriteHeader(404)
		return
	}

	_, user := getUserSession(w, r)
	followed := false

	if user.ID != 0 {
		rows, err := db.Query("select 1 from follower where follower.who_id = ? and follower.whom_id = ?", user.ID, profile_user[0]["user_id"].(int64))
		res := ctrl.HandleQuery(rows, err)
		followed = res != nil || len(res) != 0
	}

	messages, err := getMessages(w, r, true)

	if err != nil {
		w.WriteHeader(500)
		return
	}

	data := TimelineData{
		RequestUrl:   r.URL.Path,
		Followed:     followed,
		Messages:     messages,
		Profile_User: ctrl.User{Username: profile_user[0]["username"].(string)},
		SessionData:  SessionData{User: ctrl.User{Username: user.Username}},
	}

	tmpl := setupTimelineTemplates(data)
	tmpl.Execute(w, data)
}

func follow(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)

	session, user := getUserSession(w, r)
	if user.Username == "" {
		w.WriteHeader(401)
		return
	}
	vars := mux.Vars(r)
	whom_id := ctrl.GetUserID(vars["username"], db)
	if whom_id == -1 {
		w.WriteHeader(404)
		return
	}
	_, err := db.Exec("insert into follower (who_id, whom_id) values (?, ?)", user.ID, whom_id)
	ctrl.CheckError(err)
	session.AddFlash("You are now following %s", vars["username"])
	str := "/" + vars["username"]
	http.Redirect(w, r, str, http.StatusSeeOther)
}

func unfollow(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)

	session, user := getUserSession(w, r)
	if user.Username == "" {
		w.WriteHeader(401)
		return
	}
	vars := mux.Vars(r)
	whom_id := ctrl.GetUserID(vars["username"], db)
	if whom_id == -1 {
		w.WriteHeader(404)
		return
	}
	_, err := db.Exec("delete from follower where who_id=? and whom_id=?", user.ID, whom_id)
	ctrl.CheckError(err)
	session.AddFlash("You are no longer following %s", vars["username"])
	session.Save(r, w)
	str := "/" + vars["username"]
	http.Redirect(w, r, str, http.StatusSeeOther)
}

func addMessage(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	session, user := getUserSession(w, r)

	if user.ID == 0 {
		w.WriteHeader(401)
		return
	}
	if r.FormValue("text") != "" {
		_, err := db.Exec("insert into message (author_id, text, pub_date, flagged) values (?, ?, ?, 0)", user.ID, r.FormValue("text"), int(time.Now().Unix()))
		ctrl.CheckError(err)
		session.AddFlash("Your message was recorded")
		session.Save(r, w)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func login(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)

	session, user := getUserSession(w, r)
	user_id := user.ID
	username := user.Username
	if user_id != 0 {
		fmt.Println("user_id is", user_id)   // delete later
		fmt.Println("username is", username) // delete later
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	var error string
	if r.Method == "POST" {
		inputUsername := r.FormValue("username")
		inputPassword := r.FormValue("password")

		rows, err := db.Query("select * from user where username = ?", inputUsername)
		user := ctrl.HandleQuery(rows, err)

		if user == nil {
			error = "Invalid username"
		} else if !checkPwHash(inputPassword, user[0]["pw_hash"].(string)) {
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
		} else if ctrl.GetUserID(r.FormValue("username"), db) != -1 {
			error = "The username is already taken"
		} else {
			hashed_pw, err := ctrl.HashPw(r.FormValue("password"))
			ctrl.CheckError(err)
			_, err = db.Exec("insert into user (username, email, pw_hash) values (?, ?, ?)", r.FormValue("username"), r.FormValue("email"), hashed_pw)
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
func checkPwHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
