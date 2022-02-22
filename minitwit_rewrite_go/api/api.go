package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	mt "minitwit_rewrite"
)

var LATEST int = 0

func main() {
	r := mux.NewRouter()

	//r.HandleFunc("/api", timeline)
	r.HandleFunc("/api/register", register)
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
	rv := mt.Query_db("SELECT user.user_id FROM user WHERE username = ?", []interface{}{username}, true)

	if rv != nil {
		return rv[0]["user_id"].(int)
	}

	return -1
}

func update_latest(r *http.Request) {
	def := -1
	vars := mux.Vars(r)
	val := def
	if len(vars) != 0 {
		val, _ = strconv.Atoi(vars["latest"])
	}
	LATEST = val
}

func get_latest() ([]byte, error) {
	return json.Marshal(LATEST)
}

func register(w http.ResponseWriter, r *http.Request) {
	update_latest(r)

	request_data := json.NewDecoder(r.Body)

	r_data := struct {
		username string `json:"username"`
		email    string `json:"email"`
		pwd      string `json:"pwd"`
	}{}

	request_data.Decode(&r_data)

	var error string
	if r.Method == "POST" {
		if r_data.username == "" {
			error = "You have to enter a username"
		} else if r_data.email == "" || !strings.Contains(r_data.email, "@") {
			error = "You have to enter a valid email address"
		} else if r_data.pwd == "" {
			error = "You have to enter a password"
		} else if get_user_id(r_data.username) == -1 {
			error = "The username is already taken"
		} else {
			db := mt.Connect_db()
			hashed_pw, err := mt.Generate_password_hash(r_data.pwd)
			mt.CheckError(err)
			query := "INSERT INTO user (username, email, pw_hash) VALUES (?, ?, ?)"
			rv, err := db.Query(query, r_data.username, r_data.email, hashed_pw)
			mt.CheckError(err)
			defer rv.Close()
		}
	}

	if error != "" {
		fmt.Println(json.MarshalIndent(struct {
			status    int
			error_msg string
		}{400, error}, "", "\t"))
	} else {
		fmt.Println("", 204)
	}
}

func messages(w http.ResponseWriter, r *http.Request) []byte {
	update_latest(r)

	not_from_sim_response := not_req_from_simulator(w, r)
	if not_from_sim_response != nil {
		return not_from_sim_response
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
		messages := mt.Query_db(query, []interface{}{no_msgs}, true)

		// Jeg har ingen idé om det her virker som det skal, eller overhovedet..
		var filtered_msgs []mt.Message
		for m := range messages {
			var filtered_msg mt.Message
			filtered_msg.Text = m["text"].(string)
			filtered_msg.Pub_date = m["pub_date"].(int)
			filtered_msg.Author_id = m["user_id"].(int)
			filtered_msgs = append(filtered_msgs, filtered_msg)
		}

		resp, _ := json.Marshal(filtered_msgs)

		return resp
	}
}

func messages_per_user(w http.ResponseWriter, r *http.Request) []byte {
	update_latest(r)

	not_from_sim_response := not_req_from_simulator(w, r)

	if not_from_sim_response != nil {
		return not_from_sim_response
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
		messages := mt.Query_db(query, []interface{}{no_msgs}, true)

		var filtered_msgs []mt.Message

		for _, m := range messages {
			filtered_msg := mt.Message{
				Message_id: m["message_id"].(int),
				Author_id:  m["author_id"].(int),
				Text:       m["text"].(string),
				Pub_date:   m["pub_date"].(int),
				Flagged:    m["flagged"].(int),
			}

			filtered_msgs = append(filtered_msgs, filtered_msg)
		}

		response, _ := json.Marshal(filtered_msgs)
		return response
	} else if r.Method == "POST" {
		return nil
	}
}
