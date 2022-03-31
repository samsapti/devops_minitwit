package main

import (
	"encoding/json"
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
	Status int
}

var (
	db     *gorm.DB
	latest = 0
)

const (
	port = 8000
)

func main() {
	db = ctrl.ConnectDB(ctrl.DBPath)
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

func notReqFromSimulator(w http.ResponseWriter, r *http.Request) []byte {
	if r.Header.Get("Authorization") != os.Getenv("SIM_AUTH") {
		w.Header().Set("Content-Type", "application/json")

		response, _ := json.Marshal(map[string]interface{}{
			"status": http.StatusForbidden,
			"error":  "You are not authorized to use this resource!",
		})

		w.Write(response)
		return response
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

			ctrl.CheckError(err)
		}
	}

	log.Println(errorMsg)

	response, _ := json.Marshal(Response{Status: status})
	w.WriteHeader(status)
	w.Write(response)
}

func messages(w http.ResponseWriter, r *http.Request) {
	updateLatest(r)
	notFromSimResponse := notReqFromSimulator(w, r)

	if notFromSimResponse != nil {
		w.WriteHeader(403)
		w.Write(notFromSimResponse)
		return
	}

	def := 100
	vars := mux.Vars(r)
	val := def

	if len(vars) != 0 {
		val, _ = strconv.Atoi(vars["no"])
	}

	noMsgs := val

	if r.Method == "GET" {
		var messages []ctrl.Message
		db.Limit(noMsgs).Order("messages.date desc").Joins("users").Where("flagged = ?", 0).Find(&messages)

		log.Println(len(messages))
		response, _ := json.Marshal(messages)
		w.WriteHeader(200)
		w.Write(response)
	} else {
		w.WriteHeader(405) // Method Not Allowed
	}
}

func messagesPerUser(w http.ResponseWriter, r *http.Request) {
	updateLatest(r)
	notFromSimResponse := notReqFromSimulator(w, r)

	if notFromSimResponse != nil {
		w.WriteHeader(403)
		w.Write(notFromSimResponse)
	}

	def := 100
	vars := mux.Vars(r)
	val := def

	if len(vars) != 0 {
		val, _ = strconv.Atoi(vars["no"])
	}

	noMsgs := val
	userID := ctrl.GetUserID(vars["username"], db)

	if userID == 0 {
		response, _ := json.Marshal(Response{Status: 404})
		w.WriteHeader(404)
		w.Write(response)
		return
	}

	if r.Method == "GET" {
		var messages []ctrl.Message
		db.Limit(noMsgs).Order("messages.date desc").Joins("users").Where(&ctrl.Message{AuthorID: userID, Flagged: 0}).Find(&messages)

		log.Println(len(messages))
	} else if r.Method == "POST" {
		log.Println("TWEET:")

		reqData := struct {
			Content string `json:"content"`
		}{}

		json.NewDecoder(r.Body).Decode(&reqData)

		db.Create(&ctrl.Message{
			AuthorID: userID,
			Text:     reqData.Content,
			Date:     time.Now().Unix(),
			Flagged:  0,
		})
	} else {
		w.WriteHeader(405) // Method Not Allowed
	}

	response, _ := json.Marshal(Response{Status: 204})
	w.WriteHeader(204)
	w.Write(response)
}

func follow(w http.ResponseWriter, r *http.Request) {
	log.Println("FOLLOW/UNFOLLOW:")

	updateLatest(r)
	notFromSimResponse := notReqFromSimulator(w, r)

	if notFromSimResponse != nil {
		w.WriteHeader(403)
		w.Write(notFromSimResponse)
		return
	}

	var status int
	userID := ctrl.GetUserID(mux.Vars(r)["username"], db)

	if userID == 0 {
		status := 404
		response, _ := json.Marshal(Response{Status: status})
		w.WriteHeader(status)
		w.Write(response)
		return
	}

	reqData := struct {
		Follow   string `json:"follow"`
		Unfollow string `json:"unfollow"`
	}{}

	json.NewDecoder(r.Body).Decode(&reqData)

	if len(reqData.Follow) != 0 && r.Method == "POST" {
		followID := ctrl.GetUserID(reqData.Follow, db)

		if followID == 0 {
			status := 404
			resp, _ := json.Marshal(Response{Status: status})
			w.WriteHeader(status)
			w.Write(resp)
			return
		}

		db.Create(&ctrl.Follower{FollowerID: userID, FollowedID: followID})
		status = 204
	} else if len(reqData.Unfollow) != 0 && r.Method == "POST" {
		unfollowID := ctrl.GetUserID(reqData.Unfollow, db)

		if unfollowID == 0 {
			resp, _ := json.Marshal(Response{Status: 404})
			w.WriteHeader(404)
			w.Write(resp)
			return
		}

		db.Delete(&ctrl.Follower{FollowerID: userID, FollowedID: unfollowID})
		status = 204
	} else if r.Method == "GET" {
		def := 100
		vars := mux.Vars(r)
		val := def

		if len(vars) != 0 {
			val, _ = strconv.Atoi(vars["no"])
		}

		query := "SELECT user.username FROM user INNER JOIN follower ON follower.whom_id=user.user_id WHERE follower.who_id=? LIMIT ?"
		var followers []map[string]interface{}
		if rows, err := db.Query(query, userID, val); err != nil {
			resp, _ := json.Marshal(Response{Status: 403})
			w.WriteHeader(403)
			w.Write(resp)
			return
		} else {
			followers = ctrl.HandleQuery(rows, err)
		}

		var follower_names []interface{}
		for f := range followers {
			follower_names = append(follower_names, f)
		}

		followers_response := struct {
			Follows []interface{} `json:"follows"`
		}{
			Follows: follower_names,
		}

		status = 204
	}

	response, _ := json.Marshal(Response{Status: status})
	w.WriteHeader(status)
	w.Write(response)
}
