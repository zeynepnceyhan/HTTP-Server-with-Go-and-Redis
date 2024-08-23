package match

import (
	"context"
	"encoding/json"
	"fmt"
	"masomointern/internal/constants"
	"masomointern/internal/user"
	"net/http"
	"strconv"

	"github.com/go-redis/redis/v8"
)

type Response struct {
	Status  bool        `json:"status"`
	Result  interface{} `json:"result"`
	Message string      `json:"message"`
}

type connection struct {
	RDB *redis.Client
	CTX context.Context
}

// addScoreToUser, kullanıcıya belirtilen puanı ekler
func (c *connection) addScoreToUser(userid string, score float64) {
	c.RDB.ZIncrBy(c.CTX, constants.Leaderboard, score, userid)
}

// MatchResultHandler, maç sonucunu işler ve puanları günceller
func MatchResultHandler(rdb *redis.Client, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var con connection
		con.RDB = rdb
		con.CTX = ctx

		var matchData struct {
			UserID1 int `json:"userid1"`
			UserID2 int `json:"userid2"`
			Score1  int `json:"score1"`
			Score2  int `json:"score2"`
		}

		err := json.NewDecoder(r.Body).Decode(&matchData)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		userID1 := strconv.Itoa(matchData.UserID1)
		userID2 := strconv.Itoa(matchData.UserID2)
		point1 := 1
		point2 := 1

		if matchData.Score1 > matchData.Score2 {
			point1 = 3
			point2 = 0
		} else if matchData.Score1 < matchData.Score2 {
			point1 = 0
			point2 = 3
		}

		con.addScoreToUser(userID1, float64(point1))
		con.addScoreToUser(userID2, float64(point2))

		user1, err := user.GetUserByID(rdb, ctx, matchData.UserID1)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		user2, err := user.GetUserByID(rdb, ctx, matchData.UserID2)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Println(user1, user2)

		json.NewEncoder(w).Encode(Response{Status: true, Result: true})
	}
}

// LeaderboardHandler, sıralamayı ve puanları döndürür
func LeaderboardHandler(rdb *redis.Client, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		pageStr := r.URL.Query().Get("page")
		countStr := r.URL.Query().Get("count")
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}

		count, err := strconv.Atoi(countStr)
		if err != nil || count < 1 {
			count = 10
		}

		start := (page - 1) * count
		end := start + count - 1

		users, err := rdb.ZRevRangeWithScores(ctx, constants.Leaderboard, int64(start), int64(end)).Result()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leaderboard := make([]map[string]interface{}, len(users))
		for i, redisUser := range users {
			userID, err := strconv.Atoi(redisUser.Member.(string))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			u, err := user.GetUserByID(rdb, ctx, userID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			leaderboard[i] = map[string]interface{}{
				"id":       u.ID,
				"username": u.Username,
				"rank":     start + i + 1,
				"score":    redisUser.Score,
			}
		}

		json.NewEncoder(w).Encode(Response{Status: true, Result: leaderboard})
	}
}
