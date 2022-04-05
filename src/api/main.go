package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	ctrl "minitwit/controllers"
	mntr "minitwit/monitoring"
)

type Response struct {
	Status   int
	ErrorMsg string
}

var (
	db     *gorm.DB
	latest = 0
)

const (
	port = 8000
)

func main() {
	db = ctrl.ConnectDB()
	r := mux.NewRouter()

	// Endpoints
	r.HandleFunc("/api/latest", getLatest)
	r.HandleFunc("/api/register", register)
	r.HandleFunc("/api/fllws/{username}", follow)
	r.HandleFunc("/api/msgs/{username}", messagesPerUser)
	r.HandleFunc("/api/msgs", messages)

	// Register r as HTTP handler
	http.Handle("/", mntr.MiddlewareMetrics(r, true))

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
		Start API server
	*/

	srv := &http.Server{
		Addr:         "0.0.0.0:" + strconv.Itoa(port),
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}

	log.Printf("Starting API on port %d\n", port)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal("Error: ", err)
	}
}

func notReqFromSimulator(w http.ResponseWriter, r *http.Request) *Response {
	if r.Header.Get("Authorization") != os.Getenv("SIM_AUTH") {
		w.Header().Set("Content-Type", "application/json")
		status := 403

		return &Response{
			Status:   status,
			ErrorMsg: "You are not authorized to use this resource!",
		}
	}

	return nil
}

func updateLatest(r *http.Request) {
	params := r.URL.Query()
	def := -1
	val := def

	if params.Get("latest") != "" {
		val, _ = strconv.Atoi(params.Get("latest"))
	}

	if val != def {
		latest = val
	}
}

func getLatest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	resp, _ := json.Marshal(struct {
		Latest int `json:"latest"`
	}{latest})

	w.Write(resp)
	w.WriteHeader(200)
}

func register(w http.ResponseWriter, r *http.Request) {
	log.Println("REGISTER:")
	updateLatest(r)

	reqData := struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Pwd      string `json:"pwd"`
	}{}

	json.NewDecoder(r.Body).Decode(&reqData)

	var status int
	var errorMsg string

	if r.Method == "POST" {
		if len(reqData.Username) == 0 {
			errorMsg = "You have to enter a username"
			status = 400
		} else if len(reqData.Email) == 0 || !strings.Contains(reqData.Email, "@") {
			errorMsg = "You have to enter a valid email address"
			status = 400
		} else if len(reqData.Pwd) == 0 {
			errorMsg = "You have to enter a password"
			status = 400
		} else if ctrl.GetUserID(reqData.Username, db) != 0 {
			errorMsg = "The username is already taken"
			status = 400
		} else {
			status = 204
			pw, err := ctrl.HashPw(reqData.Pwd)
			ctrl.CheckError(err)

			db.Create(&ctrl.User{
				Username: reqData.Username,
				Email:    reqData.Email,
				PwHash:   pw,
			})

			log.Println("Created" + reqData.Username)
			ctrl.CheckError(err)
		}

		if status == 400 {
			response, _ := json.Marshal(&Response{
				Status:   status,
				ErrorMsg: errorMsg,
			})

			w.Write(response)
		}
	} else {
		status = 405 // Method Not Allowed
	}

	w.WriteHeader(status)
}

func messages(w http.ResponseWriter, r *http.Request) {
	updateLatest(r)
	notFromSimResponse := notReqFromSimulator(w, r)

	if notFromSimResponse != nil {
		response, _ := json.Marshal(notFromSimResponse)
		w.WriteHeader(notFromSimResponse.Status)
		w.Write(response)
		return
	}

	status := 200
	def := 100
	vars := mux.Vars(r)
	val := def

	if len(vars) != 0 {
		val, _ = strconv.Atoi(vars["no"])
	}

	noMsgs := val

	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		var messages []ctrl.Message

		query := db.Limit(noMsgs).
			Joins("JOIN user ON message.author_id = user.user_id").
			Order("message.pub_date desc").
			Where("flagged = ?", 0).
			Find(&messages)

		if query.Error != nil && !errors.Is(query.Error, gorm.ErrRecordNotFound) {
			status = 500
		} else {
			response, _ := json.Marshal(messages)
			w.Write(response)
		}
	} else {
		status = 405 // Method Not Allowed
	}

	w.WriteHeader(status)
}

func messagesPerUser(w http.ResponseWriter, r *http.Request) {
	updateLatest(r)
	notFromSimResponse := notReqFromSimulator(w, r)

	if notFromSimResponse != nil {
		response, _ := json.Marshal(notFromSimResponse)
		w.WriteHeader(notFromSimResponse.Status)
		w.Write(response)
		return
	}

	var status int
	def := 100
	vars := mux.Vars(r)
	noMsgs := def

	if len(vars) != 0 {
		noMsgs, _ = strconv.Atoi(vars["no"])
	}

	userID := ctrl.GetUserID(vars["username"], db)

	if userID == 0 {
		w.WriteHeader(404)
		return
	}

	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		status = 200

		var messages []ctrl.Message

		db.Limit(noMsgs).
			Joins("JOIN user ON message.author_id = user.user_id").
			Order("message.pub_date desc").
			Where(&ctrl.Message{AuthorID: int(userID), Flagged: 0}).
			Find(&messages)

		log.Println(len(messages))

		response, _ := json.Marshal(messages)
		w.Write(response)
	} else if r.Method == "POST" {
		log.Println("TWIT:")

		status = 204

		reqData := struct {
			Content string `json:"content"`
		}{}

		json.NewDecoder(r.Body).Decode(&reqData)

		db.Create(&ctrl.Message{
			AuthorID: int(userID),
			Text:     reqData.Content,
			Date:     time.Now().Unix(),
			Flagged:  0,
		})
	} else {
		status = 405 // Method Not Allowed
	}

	w.WriteHeader(status)
}

func follow(w http.ResponseWriter, r *http.Request) {
	log.Println("FOLLOW/UNFOLLOW:")

	updateLatest(r)
	notFromSimResponse := notReqFromSimulator(w, r)

	if notFromSimResponse != nil {
		response, _ := json.Marshal(notFromSimResponse)
		w.WriteHeader(notFromSimResponse.Status)
		w.Write(response)
		return
	}

	var status int
	userID := ctrl.GetUserID(mux.Vars(r)["username"], db)

	if userID == 0 {
		w.WriteHeader(404)
		return
	}

	reqData := struct {
		Follow   string `json:"follow"`
		Unfollow string `json:"unfollow"`
	}{}

	json.NewDecoder(r.Body).Decode(&reqData)

	if len(reqData.Follow) != 0 && r.Method == "POST" {
		status = 204
		followID := ctrl.GetUserID(reqData.Follow, db)

		if followID == 0 {
			status = 404
		} else {
			query := db.Debug().FirstOrCreate(&ctrl.Follower{}, &ctrl.Follower{
				FollowerID: userID,
				FollowsID:  followID,
			})

			if query.Error != nil && !errors.Is(query.Error, gorm.ErrRecordNotFound) {
				status = 500
			}
		}
	} else if len(reqData.Unfollow) != 0 && r.Method == "POST" {
		status = 204
		unfollowID := ctrl.GetUserID(reqData.Unfollow, db)

		if unfollowID == 0 {
			w.WriteHeader(404)
			return
		}

		query := db.Delete(&ctrl.Follower{
			FollowerID: userID,
			FollowsID:  unfollowID,
		})

		if query.Error != nil && !errors.Is(query.Error, gorm.ErrRecordNotFound) {
			status = 500
		}
	} else if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		status = 200

		var followers []ctrl.User
		var followerNames []interface{}

		query := db.Debug().Select("user.username").Joins("INNER JOIN follower ON user.user_id = follower.who_id").
			Find(&followers, "follower.whom_id = ?", userID)

		if query.Error != nil && !errors.Is(query.Error, gorm.ErrRecordNotFound) {
			status = 500
		} else {
			for _, f := range followers {
				log.Println(f.Username)
				log.Println(f.ID)
				followerNames = append(followerNames, f.Username)
			}

			response, _ := json.Marshal(struct {
				Followers []interface{} `json:"followers"`
			}{Followers: followerNames})

			w.Write(response)
		}
	}

	w.WriteHeader(status)
}
