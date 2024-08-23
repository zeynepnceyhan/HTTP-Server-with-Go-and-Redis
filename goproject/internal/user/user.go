package user

import (
	"context"
	"encoding/json"
	"errors"
	"masomointern/internal/authent"
	"masomointern/internal/constants"
	"net/http"
	"strconv"

	"github.com/go-redis/redis/v8"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Surname  string `json:"surname"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Response struct {
	Status  bool        `json:"status"`
	Result  interface{} `json:"result"`
	Message string      `json:"message"`
}

func passwordToHash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func checkHashedPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func checkUserExists(rdb *redis.Client, ctx context.Context, userID int) (bool, error) {
	key := constants.UserPrefix + strconv.Itoa(userID)
	result, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// RegisterHandler handles user registration
func RegisterHandler(rdb *redis.Client, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var newUser User
		err := json.NewDecoder(r.Body).Decode(&newUser)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		existingUsername, err := rdb.Get(ctx, constants.UsernamePrefix+newUser.Username).Result()
		if err != redis.Nil && err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if existingUsername != "" {
			json.NewEncoder(w).Encode(Response{Status: false, Message: "Username already exists!"})
			return
		}

		hashedPassword, err := passwordToHash(newUser.Password)
		if err != nil {
			json.NewEncoder(w).Encode(Response{Status: false, Message: "Failed to hash password!"})
			return
		}

		newUser.Password = hashedPassword

		id, err := rdb.Incr(ctx, constants.NextUserID).Result()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		newUser.ID = int(id)

		userJson, err := json.Marshal(newUser)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = rdb.Set(ctx, constants.UserPrefix+strconv.Itoa(newUser.ID), userJson, 0).Err()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = rdb.Set(ctx, constants.UsernamePrefix+newUser.Username, strconv.Itoa(newUser.ID), 0).Err()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Token oluşturma
		token, err := authent.GenerateToken(rdb, ctx, newUser.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		responseUser := User{
			Name:     newUser.Name,
			Surname:  newUser.Surname,
			Username: newUser.Username,
		}

		json.NewEncoder(w).Encode(Response{Status: true, Result: map[string]interface{}{"user": responseUser, "token": token}})
	}
}

// LoginHandler handles user login
func LoginHandler(rdb *redis.Client, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var loginDetails User
		err := json.NewDecoder(r.Body).Decode(&loginDetails)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		idRedis, err := rdb.Get(ctx, constants.UsernamePrefix+loginDetails.Username).Result()
		if err != redis.Nil && err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if idRedis == "" {
			json.NewEncoder(w).Encode(Response{Status: false, Message: "User not found!"})
			return
		}

		id, err := strconv.Atoi(idRedis)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		userJSON, err := rdb.Get(ctx, constants.UserPrefix+strconv.Itoa(id)).Result()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var user User
		err = json.Unmarshal([]byte(userJSON), &user)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if !checkHashedPassword(loginDetails.Password, user.Password) {
			json.NewEncoder(w).Encode(Response{Status: false, Message: "Invalid password!"})
			return
		}

		// Token oluşturma
		token, err := authent.GenerateToken(rdb, ctx, user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		responseUser := User{
			Name:     user.Name,
			Surname:  user.Surname,
			Username: user.Username,
		}

		json.NewEncoder(w).Encode(Response{Status: true, Result: map[string]interface{}{"user": responseUser, "token": token}})
	}
}

func UserDetailsHandler(rdb *redis.Client, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		query := r.URL.Query()
		idRedis := query.Get("id")

		if idRedis == "" {
			json.NewEncoder(w).Encode(Response{Status: false, Message: "ID is required"})
			return
		}

		userJson, err := rdb.Get(ctx, "user:"+idRedis).Result()
		if err != nil {
			http.Error(w, "User not found!", http.StatusNotFound)
			return
		}

		var user User
		err = json.Unmarshal([]byte(userJson), &user)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		responseUser := User{
			Name:     user.Name,
			Surname:  user.Surname,
			Username: user.Username,
		}

		json.NewEncoder(w).Encode(Response{Status: true, Result: responseUser})
	}
}

func UpdateInfoHandler(rdb *redis.Client, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Token'dan kullanıcı ID'sini al
		userID, err := authent.GetUserIDFromToken(rdb, ctx, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		var updatedUser User
		err = json.NewDecoder(r.Body).Decode(&updatedUser)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if userID != updatedUser.ID {
			http.Error(w, "Cannot change another user's information", http.StatusForbidden)
			return
		}

		// Kullanıcı var mı kontrol et
		exists, err := checkUserExists(rdb, ctx, updatedUser.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !exists {
			http.Error(w, "User not found!", http.StatusNotFound)
			return
		}

		existingUserJson, err := rdb.Get(ctx, constants.UserPrefix+strconv.Itoa(updatedUser.ID)).Result()
		if err == redis.Nil {
			http.Error(w, "User not found!", http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var existingUser User
		err = json.Unmarshal([]byte(existingUserJson), &existingUser)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Kullanıcı adı güncelleme ve çakışma kontrolü
		if updatedUser.Username != "" && updatedUser.Username != existingUser.Username {
			existingUsername, err := rdb.Get(ctx, constants.UsernamePrefix+updatedUser.Username).Result()
			if err != redis.Nil && err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if existingUsername != "" {
				json.NewEncoder(w).Encode(Response{Status: false, Message: "Username already exists!"})
				return
			}

			err = rdb.Set(ctx, constants.UsernamePrefix+updatedUser.Username, strconv.Itoa(updatedUser.ID), 0).Err()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			err = rdb.Del(ctx, constants.UsernamePrefix+existingUser.Username).Err()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Diğer alanların güncellenmesi
		existingUser.Name = updatedUser.Name
		existingUser.Surname = updatedUser.Surname
		existingUser.Username = updatedUser.Username

		if updatedUser.Password != "" {
			hashedPassword, err := passwordToHash(updatedUser.Password)
			if err != nil {
				json.NewEncoder(w).Encode(Response{Status: false, Message: "Failed to hash password!"})
				return
			}
			existingUser.Password = hashedPassword
		}

		updatedUserJson, err := json.Marshal(existingUser)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = rdb.Set(ctx, constants.UserPrefix+strconv.Itoa(existingUser.ID), updatedUserJson, 0).Err()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(Response{Status: true, Result: existingUser})
	}
}

func GetUserByID(rdb *redis.Client, ctx context.Context, id int) (User, error) {
	userJson, err := rdb.Get(ctx, constants.UserPrefix+strconv.Itoa(id)).Result() //constants
	if err == redis.Nil {
		return User{}, errors.New("User not found")
	} else if err != nil {
		return User{}, err
	}

	var u User
	err = json.Unmarshal([]byte(userJson), &u)
	if err != nil {
		return User{}, err
	}

	return u, nil
}

func SaveUser(rdb *redis.Client, ctx context.Context, user User) error {
	userJSON, err := json.Marshal(user)
	if err != nil {
		return err
	}

	err = rdb.Set(ctx, constants.UserPrefix+strconv.Itoa(user.ID), userJSON, 0).Err() //constants
	if err != nil {
		return err
	}

	return nil
}
