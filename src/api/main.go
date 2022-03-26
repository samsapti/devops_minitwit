package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	ctrl "minitwit/controllers"
)

type Response struct {
	Status int
}

var DATABASE, INIT_DB_SCHEMA string = "/tmp/minitwit.db", "../../db_init.sql"
var LATEST int = 0
var db *sql.DB

const port int = 8000

func main() {
	ctrl.Init_db(INIT_DB_SCHEMA, DATABASE)

	r := mux.NewRouter()
	r.Use()

	r.HandleFunc("/api/latest", get_latest)
	r.HandleFunc("/api/register", register)
	r.HandleFunc("/api/fllws/{username}", follow)
	r.HandleFunc("/api/msgs/{username}", messages_per_user)
	r.HandleFunc("/api/msgs", messages)
	r.HandleFunc("/api/msgs/{username}", messages_per_user)

	http.Handle("/", ctrl.MiddlewareMetrics(r))

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
	db = ctrl.Connect_db(DATABASE)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal("Error: ", err)
	}
}

func logQueryInfo(res sql.Result, query string, queryData string) {
	log.Printf(query, queryData)
	affected, _ := res.RowsAffected()
	lastInsert, _ := res.LastInsertId()
	log.Printf("	affected rows: %d, LastInsertId: %d", affected, lastInsert)
}

func not_req_from_simulator(w http.ResponseWriter, r *http.Request) []byte {
	from_simulator := r.Header.Get("Authorization")
	if from_simulator != "Basic c2ltdWxhdG9yOnN1cGVyX3NhZmUh" {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"status": http.StatusForbidden,
			"error":  "You are not authorized to use this resource!",
		}
		resp, _ := json.Marshal(response)
		w.Write(resp)
		return resp

	}
	return nil
}

func get_user_id(username string) int {
	rows, err := db.Query("SELECT user.user_id FROM user WHERE username = ?", username)
	rv := ctrl.HandleQuery(rows, err)

	if rv != nil || len(rv) != 0 {
		return int(rv[0]["user_id"].(int64))
	}

	return -1
}

func update_latest(r *http.Request) {
	params := r.URL.Query()
	def := -1
	val := def
	if params.Get("latest") != "" {
		val, _ = strconv.Atoi(params.Get("latest"))
	}

	if val != -1 {
		LATEST = val
	}
}
func get_latest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	latest_struct := struct {
		Latest int `json:"latest"`
	}{
		LATEST,
	}
	resp, _ := json.Marshal(latest_struct)
	w.Write(resp)
}

func register(w http.ResponseWriter, r *http.Request) {
	log.Println("REGISTER:")
	update_latest(r)

	request_data := json.NewDecoder(r.Body)

	r_data := struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Pwd      string `json:"pwd"`
	}{}

	request_data.Decode(&r_data)
	var status int
	var errorMsg string
	if r.Method == "POST" {
		if r_data.Username == "" {
			errorMsg = "You have to enter a username"
			status = 400
		} else if r_data.Email == "" || !strings.Contains(r_data.Email, "@") {
			errorMsg = "You have to enter a valid email address"
			status = 400
		} else if r_data.Pwd == "" {
			errorMsg = "You have to enter a password"
			status = 400
		} else if get_user_id(r_data.Username) != -1 {
			errorMsg = "The username is already taken"
			status = 400
		} else {
			status = 204
			db := ctrl.Connect_db(DATABASE)
			hashed_pw, err := ctrl.Generate_password_hash(r_data.Pwd)
			ctrl.CheckError(err)

			query := "INSERT INTO user (username, email, pw_hash) VALUES (?, ?, ?)"
			res, err := db.Exec(query, r_data.Username, r_data.Email, hashed_pw)
			ctrl.CheckError(err)
			logQueryInfo(res, "	Inserting user \"%s\" into database\n", r_data.Username)
		}
	}
	log.Println(errorMsg)
	resp, _ := json.Marshal(204)
	w.WriteHeader(status)
	w.Write(resp)
}

func messages(w http.ResponseWriter, r *http.Request) {
	update_latest(r)

	not_from_sim_response := not_req_from_simulator(w, r)
	if not_from_sim_response != nil {
		w.WriteHeader(403)
		w.Write(not_from_sim_response)
	}

	def := 100
	vars := mux.Vars(r)
	val := def
	if len(vars) != 0 {
		val, _ = strconv.Atoi(vars["no"])
	}

	no_msgs := val
	if r.Method == "GET" {
		query := "SELECT message.*, user.* FROM message, user WHERE message.flagged = 0 AND message.author_id = user.user_id ORDER BY message.pub_date DESC LIMIT ?"
		rows, err := db.Query(query, no_msgs)
		messages := ctrl.HandleQuery(rows, err)

		var filtered_msgs []ctrl.Message

		for _, m := range messages {
			filtered_msg := ctrl.Message{
				Message_id: m["message_id"].(int),
				Author_id:  m["author_id"].(int),
				Text:       m["text"].(string),
				Pub_date:   m["pub_date"].(int),
				Flagged:    m["flagged"].(int),
			}

			filtered_msgs = append(filtered_msgs, filtered_msg)
		}

		resp, _ := json.Marshal(filtered_msgs)
		w.WriteHeader(200)
		w.Write(resp)
	}
}

func messages_per_user(w http.ResponseWriter, r *http.Request) {
	log.Println("TWEET:")
	update_latest(r)

	not_from_sim_response := not_req_from_simulator(w, r)

	if not_from_sim_response != nil {
		w.WriteHeader(403)
		w.Write(not_from_sim_response)
	}

	def := 100
	vars := mux.Vars(r)
	val := def
	if len(vars) != 0 {
		val, _ = strconv.Atoi(vars["no"])
	}

	no_msgs := val

	if r.Method == "GET" {
		query := "SELECT message.*, user.* FROM message, user  WHERE message.flagged = 0 AND user.user_id = message.author_id AND user.user_id = ? ORDER BY message.pub_date DESC LIMIT ?"
		rows, err := db.Query(query, no_msgs)
		messages := ctrl.HandleQuery(rows, err)

		var filtered_msgs []ctrl.Message

		for _, m := range messages {
			filtered_msg := ctrl.Message{
				Message_id: m["message_id"].(int),
				Author_id:  m["author_id"].(int),
				Text:       m["text"].(string),
				Pub_date:   m["pub_date"].(int),
				Flagged:    m["flagged"].(int),
			}

			filtered_msgs = append(filtered_msgs, filtered_msg)
		}

		resp, _ := json.Marshal(filtered_msgs)
		w.WriteHeader(204)
		w.Write(resp)
		return

	} else if r.Method == "POST" {
		r_data := struct {
			Content string `json:"content"`
		}{}

		username := mux.Vars(r)["username"]
		json.NewDecoder(r.Body).Decode(&r_data)

		rData := ctrl.Message{
			Author_id: get_user_id(username),
			Text:      r_data.Content,
			Pub_date:  int(time.Now().Unix()),
		}

		query := "INSERT INTO message (author_id, text, pub_date, flagged) VALUES (?, ?, ?, 0)"
		if res, err := db.Exec(query, rData.Author_id, rData.Text, rData.Pub_date); err != nil {
			resp, _ := json.Marshal(Response{Status: 403})
			w.WriteHeader(403)
			w.Write(resp)
			return
		} else {
			logQueryInfo(res, "	Inserting message \"%s\" into database\n", rData.Text)
			resp, _ := json.Marshal(Response{Status: 204})
			w.WriteHeader(204)
			w.Write(resp)
		}
	}
}

func follow(w http.ResponseWriter, r *http.Request) {
	log.Println("FOLLOW/UNFOLLOW:")
	username := mux.Vars(r)["username"]
	update_latest(r)
	decoder := json.NewDecoder(r.Body)

	not_from_sim_response := not_req_from_simulator(w, r)
	if not_from_sim_response != nil {
		w.WriteHeader(403)
		w.Write(not_from_sim_response)
		return
	}

	user_id := get_user_id(username)
	if user_id == -1 {
		status := 404
		resp, _ := json.Marshal(Response{Status: status})
		w.WriteHeader(status)
		w.Write(resp)
		return
	}

	type fReq struct {
		Follow   string `json:"follow"`
		Unfollow string `json:"unfollow"`
	}
	req := fReq{}
	decoder.Decode(&req)

	if req.Follow != "" && r.Method == "POST" {
		follows_user_id := get_user_id(req.Follow)
		if follows_user_id == -1 {
			status := 404
			resp, _ := json.Marshal(Response{Status: status})
			w.WriteHeader(status)
			w.Write(resp)
			return
		}

		query := "INSERT INTO follower (who_id, whom_id) VALUES (?, ?)"
		if res, err := db.Exec(query, user_id, follows_user_id); err != nil {
			resp, _ := json.Marshal(Response{Status: 403})
			w.WriteHeader(403)
			w.Write(resp)
			return
		} else {
			logQueryInfo(res, "	Inserting follower \"%s\" into database\n", req.Follow)
			resp, _ := json.Marshal(Response{Status: 204})
			w.WriteHeader(204)
			w.Write(resp)
		}

		return
	} else if req.Unfollow != "" && r.Method == "POST" {
		unfollows_username := req.Unfollow
		unfollows_user_id := get_user_id(unfollows_username)

		if unfollows_user_id == -1 {
			resp, _ := json.Marshal(Response{Status: 404})
			w.WriteHeader(404)
			w.Write(resp)
			return
		}

		query := "DELETE FROM follower WHERE who_id=? and WHOM_ID=?"
		if res, err := db.Exec(query, user_id, unfollows_user_id); err != nil {
			resp, _ := json.Marshal(Response{Status: 403})
			w.WriteHeader(403)
			w.Write(resp)
			return
		} else {
			logQueryInfo(res, "	Deleting follower \"%s\" from database\n", unfollows_username)
			resp, _ := json.Marshal(Response{Status: 204})
			w.WriteHeader(204)
			w.Write(resp)
		}

		return
	} else if r.Method == "GET" {
		def := 100
		vars := mux.Vars(r)
		val := def
		if len(vars) != 0 {
			val, _ = strconv.Atoi(vars["no"])
		}

		query := "SELECT user.username FROM user INNER JOIN follower ON follower.whom_id=user.user_id WHERE follower.who_id=? LIMIT ?"
		var followers []map[string]interface{}
		if rows, err := db.Query(query, user_id, val); err != nil {
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

		resp, _ := json.Marshal(followers_response)
		w.WriteHeader(204)
		w.Write(resp)
	}
}
