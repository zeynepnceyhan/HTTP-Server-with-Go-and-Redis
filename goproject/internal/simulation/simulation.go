package simulation

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"masomointern/internal/constants"
	"masomointern/internal/user"

	"github.com/go-redis/redis/v8"
	"golang.org/x/exp/rand"
)

type Response struct {
	Status  bool        `json:"status"`
	Result  interface{} `json:"result"`
	Message string      `json:"message"`
}

type Connection struct {
	RDB *redis.Client   // Redis istemci nesnesi
	CTX context.Context // Context nesnesi
}

func (c *Connection) AddScoreToUser(userID string, score float64) error {
	return c.RDB.ZIncrBy(c.CTX, constants.Leaderboard, score, userID).Err()
}

type SimMatchData struct {
	UserID1 int `json:"userid1"`
	UserID2 int `json:"userid2"`
	Score1  int `json:"score1"`
	Score2  int `json:"score2"`
}

func SimulateMatches(rdb *redis.Client, ctx context.Context, users []user.User) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userCount := len(users)
		var con Connection
		con.RDB = rdb
		con.CTX = ctx

		var matches []SimMatchData

		for i := 0; i < userCount; i++ {
			for j := i + 1; j < userCount; j++ {
				score1 := rand.Intn(5)
				score2 := rand.Intn(5)

				match := SimMatchData{
					UserID1: users[i].ID,
					UserID2: users[j].ID,
					Score1:  score1,
					Score2:  score2,
				}

				matches = append(matches, match)

				if score1 > score2 {
					con.AddScoreToUser(strconv.Itoa(users[i].ID), 3)
				} else if score1 < score2 {
					con.AddScoreToUser(strconv.Itoa(users[j].ID), 3)
				} else {
					con.AddScoreToUser(strconv.Itoa(users[i].ID), 1)
					con.AddScoreToUser(strconv.Itoa(users[j].ID), 1)
				}
			}
		}

		response := map[string]interface{}{
			"matches": matches,
			"message": "Simulation completed successfully",
		}

		json.NewEncoder(w).Encode(Response{Status: true, Result: response})
	}
}

func SimulationHandler(rdb *redis.Client, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userCountStr := r.URL.Query().Get("usercount")
		userCount, err := strconv.Atoi(userCountStr)
		if err != nil || userCount < 1 {
			http.Error(w, "Invalid user count", http.StatusBadRequest)
			return
		}

		users, err := addUserToUserlist(rdb, ctx, userCount)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		SimulateMatches(rdb, ctx, users)(w, r)
	}
}

func addUserToUserlist(rdb *redis.Client, ctx context.Context, userCount int) ([]user.User, error) {
	users := make([]user.User, 0, userCount)

	for i := 0; i < userCount; i++ {
		userID := i + 1

		username := generateUsername("player_", userID)
		name := generateRandomName()
		password := "password"

		newUser := user.User{
			ID:       userID,
			Username: username,
			Name:     name,
			Password: password,
		}

		userJSON, err := json.Marshal(newUser)
		if err != nil {
			return nil, err
		}

		err = rdb.Set(ctx, constants.UserPrefix+strconv.Itoa(newUser.ID), userJSON, 0).Err()
		if err != nil {
			return nil, err
		}

		err = rdb.Set(ctx, constants.UsernamePrefix+newUser.Username, strconv.Itoa(newUser.ID), 0).Err()
		if err != nil {
			return nil, err
		}

		users = append(users, newUser)
	}

	return users, nil
}

func generateRandomName() string {
	firstNames := []string{"A", "B"}
	lastNames := []string{"C", "D"}
	return firstNames[rand.Intn(len(firstNames))] + " " + lastNames[rand.Intn(len(lastNames))]
}

func generateUsername(prefix string, userID int) string {
	return prefix + strconv.Itoa(userID)
}
