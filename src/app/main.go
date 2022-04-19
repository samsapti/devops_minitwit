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
	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})
	r.HandleFunc("/public", publicTimeline)
	r.HandleFunc("/add_message", addMessage).Methods("POST")
	r.HandleFunc("/login", login).Methods("GET", "POST")
	r.HandleFunc("/register", register).Methods("GET", "POST")
	r.HandleFunc("/logout", logout)
	r.HandleFunc("/{username}", userTimeline)
	r.HandleFunc("/{username}/follow", follow)
	r.HandleFunc("/{username}/unfollow", unfollow)

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

func getMessages(w http.ResponseWriter, r *http.Request, public bool, own bool) ([]ctrl.Message, error) {
	_, user := getUserSession(w, r)
	var messages []ctrl.Message

	if public {
		query := db.Limit(perPage).
			Joins("JOIN user ON message.author_id = user.user_id").
			Order("message.pub_date desc").
			Find(&messages, "flagged = ?", 0)

		if query.Error != nil && !errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return nil, query.Error
		}
	} else if own {
		subquery := db.Select("whom_id").Find(&ctrl.Follower{}, "who_id = ?", user.ID)
		query := db.Limit(perPage).
			Joins("JOIN user ON message.author_id = user.user_id").
			Order("message.pub_date desc").
			Where("user.user_id = ?", user.ID).
			Or("user.user_id IN (?)", subquery).
			Find(&messages, "flagged = ?", 0)

		if subquery.Error != nil && !errors.Is(subquery.Error, gorm.ErrRecordNotFound) {
			return nil, subquery.Error
		} else if query.Error != nil && !errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return nil, query.Error
		}
	} else {
		username := mux.Vars(r)["username"]

		query := db.Limit(perPage).
			Order("pub_date desc").
			Joins("JOIN user ON message.author_id = user.user_id").
			Find(&messages, "message.flagged = ? AND user.username = ?", 0, username)

		if query.Error != nil && !errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return nil, query.Error
		}
	}

	log.Printf("Showing %d results", len(messages))
	return messages, nil
}

func setupTimelineTemplates(data TimelineData) *template.Template {
	tmpl, err := template.New("timeline.html").Funcs(template.FuncMap{
		"gravatar_url": func(authorID uint, size int) string {
			var author ctrl.User
			db.First(&author, "user_id = ?", authorID)
			return gravatarUrl(author.Email, size)
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
		"get_username": func(id uint) string {
			var user ctrl.User
			db.First(&user, "user_id = ?", id)
			return user.Username
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

	messages, err := getMessages(w, r, false, true)

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

	messages, err := getMessages(w, r, true, false)

	if err != nil {
		log.Println(err)
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

	var profileUser ctrl.User
	queryCheck := db.First(&profileUser, "username = ?", vars["username"])

	if queryCheck.Error != nil {
		if errors.Is(queryCheck.Error, gorm.ErrRecordNotFound) {
			w.WriteHeader(404)
			return
		}

		w.WriteHeader(500)
		return
	}

	_, user := getUserSession(w, r)
	followed := true

	if user.ID != 0 {
		var follow ctrl.Follower
		query := db.First(&follow, "who_id = ? AND whom_id = ?", user.ID, profileUser.ID)

		if query.Error != nil {
			if errors.Is(query.Error, gorm.ErrRecordNotFound) {
				followed = false
			} else {
				w.WriteHeader(500)
				return
			}
		}
	}

	messages, err := getMessages(w, r, false, false)

	if err != nil {
		w.WriteHeader(500)
		return
	}

	data := TimelineData{
		RequestUrl:   r.URL.Path,
		Followed:     followed,
		Messages:     messages,
		Profile_User: ctrl.User{Username: profileUser.Username},
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
	followsID := ctrl.GetUserID(vars["username"], db)

	if followsID == 0 {
		w.WriteHeader(404)
		return
	}

	query := db.Create(&ctrl.Follower{FollowerID: user.ID, FollowsID: followsID})

	if query.Error != nil {
		w.WriteHeader(500)
		return
	}

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
	followsID := ctrl.GetUserID(vars["username"], db)

	if followsID == 0 {
		w.WriteHeader(404)
		return
	}

	query := db.Where("who_id = ? AND whom_id = ?", user.ID, followsID).Delete(&ctrl.Follower{})

	if query.Error != nil && !errors.Is(query.Error, gorm.ErrRecordNotFound) {
		w.WriteHeader(500)
		return
	}

	session.AddFlash("You are no longer following %s", vars["username"])
	session.Save(r, w)
	str := "/" + vars["username"]
	http.Redirect(w, r, str, http.StatusSeeOther)
}

func addMessage(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	session, user := getUserSession(w, r)
	text := r.FormValue("text")

	if user.ID == 0 {
		w.WriteHeader(401)
		return
	}

	if text != "" {
		query := db.Create(&ctrl.Message{
			AuthorID: user.ID,
			Text:     text,
			Date:     time.Now().Unix(),
			Flagged:  0,
		})

		if query.Error != nil {
			w.WriteHeader(500)
			return
		}

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

		var user ctrl.User
		query := db.First(&user, "username = ?", inputUsername)

		if query.Error != nil {
			if errors.Is(query.Error, gorm.ErrRecordNotFound) {
				error = "Invalid username"
			} else if !checkPwHash(inputPassword, user.PwHash) {
				error = "Invalid password"
			} else {
				error = "Something went wrong"
			}
		} else {
			session.AddFlash("You were logged in")
			session.Values["user_id"] = user.ID
			session.Values["username"] = user.Username
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
		inputUsername := r.FormValue("username")
		inputEmail := r.FormValue("email")
		inputPassword := r.FormValue("password")
		inputRepeatPassword := r.FormValue("password2")
		userID := ctrl.GetUserID(inputUsername, db)

		if inputUsername == "" {
			error = "You have to enter a username"
		} else if inputEmail == "" || !strings.Contains(inputEmail, "@") {
			error = "You have to enter a valid email address"
		} else if inputPassword == "" {
			error = "You have to enter a password"
		} else if inputPassword != inputRepeatPassword {
			error = "The two passwords do not match"
		} else if userID != 0 {
			error = "The username is already taken"
		} else {
			hashed_pw, err := ctrl.HashPw(inputPassword)
			ctrl.CheckError(err)

			query := db.Create(&ctrl.User{
				Username: inputUsername,
				Email:    inputEmail,
				PwHash:   hashed_pw,
			})

			if query.Error != nil {
				w.WriteHeader(500)
				return
			}

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
