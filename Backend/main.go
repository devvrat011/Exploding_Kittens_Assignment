package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

var rdb *redis.Client
var ctx = context.Background()

func init() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
}

type UserData struct {
	Username string `json:"username"`
	Points   int    `json:"points"`
}

func setUser(w http.ResponseWriter, r *http.Request) {
	var data map[string]string
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := data["name"]
	exists, err := rdb.Exists(ctx, name).Result()
	if err != nil {
		log.Println("Error checking if user exists:", err)
		http.Error(w, "Failed to check if user exists", http.StatusInternalServerError)
		return
	}

	if exists == 0 {
		err := rdb.ZAdd(ctx, "leaderboard", &redis.Z{Score: 0, Member: name}).Err()
		if err != nil {
			log.Println("Error setting points for new user:", err)
			http.Error(w, "Failed to set points for new user", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("User points initialized successfully"))
}

func getUserPoints(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")

	points, err := rdb.ZScore(ctx, "leaderboard", name).Result()
	if err == redis.Nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Println("Error getting user points:", err)
		http.Error(w, "Failed to get user points", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(int(points))
}

func updateUserPoints(w http.ResponseWriter, r *http.Request) {
	var data map[string]string
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := data["name"]

	newPoints, err := rdb.ZIncrBy(ctx, "leaderboard", 1, name).Result()
	if err != nil {
		log.Println("Error updating user points:", err)
		http.Error(w, "Failed to update user points", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"message": "User points updated successfully",
		"points":  int(newPoints),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getAllUserPoints(w http.ResponseWriter, r *http.Request) {
	leaderboard, err := rdb.ZRevRangeWithScores(ctx, "leaderboard", 0, -1).Result()
	if err != nil {
		log.Println("Error fetching leaderboard:", err)
		http.Error(w, "Failed to fetch leaderboard", http.StatusInternalServerError)
		return
	}

	userPointsMap := make(map[string]int)
	for _, z := range leaderboard {
		userPointsMap[z.Member.(string)] = int(z.Score)
	}

	responseJSON, err := json.Marshal(userPointsMap)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		http.Error(w, "Failed to marshal JSON", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(responseJSON); err != nil {
		log.Println("Error writing response:", err)
	}
}

func getLeaderboard(w http.ResponseWriter, r *http.Request) {
	leaderboard, err := rdb.ZRevRangeWithScores(ctx, "leaderboard", 0, 4).Result()
	if err != nil {
		log.Println("Error fetching leaderboard:", err)
		http.Error(w, "Failed to fetch leaderboard", http.StatusInternalServerError)
		return
	}

	var users []UserData
	for _, z := range leaderboard {
		users = append(users, UserData{
			Username: z.Member.(string),
			Points:   int(z.Score),
		})
	}

	responseJSON, err := json.Marshal(users)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		http.Error(w, "Failed to marshal JSON", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(responseJSON); err != nil {
		log.Println("Error writing response:", err)
	}
}

func resetUserPoints(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	err := rdb.ZAdd(ctx, "leaderboard", &redis.Z{Score: 0, Member: name}).Err()
	if err != nil {
		log.Println("Error resetting user points:", err)
		http.Error(w, "Failed to reset user points", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("User points reset successfully"))
}


func main() {
	r := mux.NewRouter()
	r.HandleFunc("/api/user", setUser).Methods("POST")
	r.HandleFunc("/api/user/points", getUserPoints).Methods("GET")
	r.HandleFunc("/api/user/points", updateUserPoints).Methods("PUT")
	r.HandleFunc("/api/user/points/all", getAllUserPoints).Methods("GET")
	r.HandleFunc("/api/leaderboard", getLeaderboard).Methods("GET")
	r.HandleFunc("/api/user/reset", resetUserPoints).Methods("PUT")

	c := cors.AllowAll()
	handler := c.Handler(r)

	log.Println("Server started on port 8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
