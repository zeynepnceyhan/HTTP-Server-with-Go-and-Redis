package friendship

import (
	"context"
	"encoding/json"
	"fmt"
	"masomointern/internal/authent"
	"masomointern/internal/constants"
	"masomointern/internal/user"

	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	DEBUG bool = true
)

// Sınırsız parametreli log fonksiyonu
func PrintLog(str ...string) {
	if DEBUG {
		fmt.Println(str)
	}
}

// FriendRequestDetails arkadaşlık isteği detayları
type FriendRequestDetails struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Date     string `json:"date"`
}

// AcceptFriendRequest arkadaşlık isteği kabul/red yapısı
type AcceptFriendRequest struct {
	RequesterID string `json:"requester_id"`
	Status      string `json:"status"`
}

// FriendDetails arkadaşlık detayları
type FriendDetails struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

// Response genel API yanıt yapısı
type Response struct {
	Status  bool        `json:"status"`
	Result  interface{} `json:"result"`
	Message string      `json:"message"`
}

// Arkadaşlık isteği yapısı
type FriendRequest struct {
	UserID string `json:"userid"`
}

type UserDetails struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

func UserSearchHandler(rdb *redis.Client, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Query().Get("username") // URL'den username al

		PrintLog("User search - taking username from URL")

		if username == "" {
			PrintLog("Username is empty")
			http.Error(w, "Username is required", http.StatusBadRequest)
			return
		}
		PrintLog("Username is not blank")

		// İstekten token'ı al ve user ID'sini elde et
		tokenUserID, err := authent.GetUserIDFromToken(rdb, ctx, r)
		if err != nil {
			PrintLog("Error getting user ID from token:", err.Error())
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		PrintLog("Token obtained, user ID retrieved:", strconv.Itoa(tokenUserID))

		// Kullanıcının kendi username'ini aratmasını engelle
		userID, err := SearchUserByUsername(rdb, ctx, username)
		if err != nil {
			PrintLog("User not found for username:", username)
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		PrintLog("User ID retrieved successfully for username:", username, "userID:", strconv.Itoa(userID))

		if userID == tokenUserID {
			PrintLog("User ID matches the searcher's ID")
			http.Error(w, "You cannot search your own username", http.StatusBadRequest)
			return
		}
		PrintLog("User ID does not match the searcher's ID")

		response := Response{
			Status: true,
			Result: userID,
		}
		PrintLog("Response created successfully")

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			PrintLog("Error encoding response:", err.Error())
			http.Error(w, "Error encoding response", http.StatusInternalServerError)
			return
		}
		PrintLog("Successful user search operation")
	}
}

// Kullanıcı adıyla arama yapar ve kullanıcı ID'sini döner
func SearchUserByUsername(rdb *redis.Client, ctx context.Context, username string) (int, error) {
	userIDStr, err := rdb.Get(ctx, constants.UsernamePrefix+username).Result()
	PrintLog("Searched with username, obtained user ID string:", userIDStr)

	if err != nil {
		PrintLog("Error getting user ID from Redis:", err.Error())
		return 0, err
	}
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		PrintLog("Error converting user ID string to integer:", err.Error())
		return 0, err
	}
	PrintLog("Converted user ID string to integer:", strconv.Itoa(userID))
	return userID, nil
}

func FriendRequestHandler(rdb *redis.Client, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		PrintLog("Friend request arrived", "path", r.URL.Path)

		// Kullanıcının kimliğini doğrula ve token'dan kullanıcı ID'sini al
		userID, err := authent.GetUserIDFromToken(rdb, ctx, r)
		if err != nil {
			PrintLog("Invalid token:", err.Error())
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		PrintLog("Token validated, user ID retrieved:", strconv.Itoa(userID))

		// İstekten hedef kullanıcı ID'sini al
		var request FriendRequest
		err = json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			msg := fmt.Sprintf("Invalid request body: %s", err)
			PrintLog("Error decoding request body:", err.Error())
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		PrintLog("Request body decoded successfully")

		targetUserID := request.UserID
		PrintLog(fmt.Sprintf("UserID: %s, TargetUserID: %s", strconv.Itoa(userID), targetUserID))

		// Önce kullanıcı ID'lerinin eşleşmesini kontrol et
		if strconv.Itoa(userID) == targetUserID {
			msg := "Cannot send friend request to oneself."
			PrintLog("Error: Cannot send friend request to oneself.")
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		// Hedef kullanıcı ID'sinin mevcut olup olmadığını kontrol et
		userKey := constants.UserPrefix + targetUserID // Anahtar oluşturulurken kullanılan prefix
		PrintLog("Redis key for target user:", userKey)
		userData, err := rdb.Get(ctx, userKey).Result()
		if err == redis.Nil {
			msg := "Target user does not exist."
			PrintLog("Error: Target user does not exist.")
			http.Error(w, msg, http.StatusNotFound)
			return
		} else if err != nil {
			msg := fmt.Sprintf("Redis error: %s", err)
			PrintLog("Error:", msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		PrintLog("User data retrieved from Redis:", userData)

		// Zaman damgasını al
		timestamp := time.Now().Unix()
		PrintLog("Timestamp obtained:", strconv.FormatInt(timestamp, 10))

		// Arkadaşlık isteğini Redis'e ekle
		err = rdb.ZAdd(ctx, constants.FriendRequestPrefix+targetUserID, &redis.Z{
			Score:  float64(timestamp),
			Member: userID,
		}).Err()
		if err != nil {
			msg := fmt.Sprintf("Failed to send friend request: %s", err)
			PrintLog("Error:", msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		PrintLog("Friend request added to Redis")

		response := Response{
			Status: true,
			Result: "Friend request sent",
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			msg := fmt.Sprintf("Failed to encode response: %s", err)
			PrintLog("Error:", msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		PrintLog("Successful friend request operation")
	}
}

func SaveUserData(rdb *redis.Client, ctx context.Context, user *user.User) error {
	key := fmt.Sprintf("username:%d", user.ID)
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, key, data, 0).Err()
}

func FriendRequestListHandler(rdb *redis.Client, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Token'dan kullanıcı ID'sini al
		userID, err := authent.GetUserIDFromToken(rdb, ctx, r)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Sayfalama parametrelerini al
		pageStr := r.URL.Query().Get("page")
		countStr := r.URL.Query().Get("count")

		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			http.Error(w, "Invalid page parameter", http.StatusBadRequest)
			return
		}

		count, err := strconv.Atoi(countStr)
		if err != nil || count < 1 {
			http.Error(w, "Invalid count parameter", http.StatusBadRequest)
			return
		}

		// Arkadaşlık isteği listelerini al
		start := (page - 1) * count
		end := start + count - 1

		friendRequests, err := rdb.ZRange(ctx, constants.FriendRequestPrefix+strconv.Itoa(userID), int64(start), int64(end)).Result()
		if err != nil {
			http.Error(w, "Error retrieving friend requests", http.StatusInternalServerError)
			return
		}

		var requests []FriendRequestDetails
		for _, requestID := range friendRequests {
			// Kullanıcı verilerini JSON formatında al
			userData, err := rdb.Get(ctx, constants.UserPrefix+requestID).Result() // Burada UsernamePrefix yerine UserPrefix kullanın
			if err != nil {
				if err == redis.Nil {
					fmt.Printf("Key %s does not exist\n", constants.UserPrefix+requestID)
				} else {
					fmt.Printf("Redis error while fetching %s: %s\n", constants.UserPrefix+requestID, err)
				}
				http.Error(w, "Error retrieving user data", http.StatusInternalServerError)
				return
			}

			// JSON verilerini ayrıştır
			var userDetails UserDetails
			err = json.Unmarshal([]byte(userData), &userDetails)
			if err != nil {
				http.Error(w, "Error parsing user data", http.StatusInternalServerError)
				return
			}

			requests = append(requests, FriendRequestDetails{
				UserID:   requestID,
				Username: userDetails.Username,
				Date:     time.Now().Format(time.RFC3339),
			})
		}

		response := Response{
			Status:  true,
			Result:  requests,
			Message: "",
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			http.Error(w, "Error encoding response", http.StatusInternalServerError)
			return
		}
	}
}

func AcceptRejectFriendRequestHandler(rdb *redis.Client, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Token'ı doğrula ve kullanıcı ID'sini al
		userID, err := authent.GetUserIDFromToken(rdb, ctx, r)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// İstek gövdesini parse et
		var request AcceptFriendRequest
		err = json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Status'un geçerli olup olmadığını kontrol et
		if request.Status != "accept" && request.Status != "reject" {
			http.Error(w, "Invalid status", http.StatusBadRequest)
			return
		}

		// Arkadaşlık isteklerini al
		friendRequestsKey := constants.FriendRequestPrefix + strconv.Itoa(userID)
		friendRequests, err := rdb.ZRange(ctx, friendRequestsKey, 0, -1).Result()
		if err != nil {
			http.Error(w, "Error retrieving friend requests", http.StatusInternalServerError)
			return
		}

		// İstekler arasında arama yap
		var found bool
		for _, requesterID := range friendRequests {
			if requesterID == request.RequesterID {
				found = true
				break
			}
		}

		if !found {
			http.Error(w, "Friend request not found", http.StatusNotFound)
			return
		}

		if request.Status == "accept" {
			// Her iki kullanıcıyı da arkadaş olarak ekle
			friendKey := constants.FriendPrefix + strconv.Itoa(userID)
			err = rdb.ZAdd(ctx, friendKey, &redis.Z{Score: float64(time.Now().Unix()), Member: request.RequesterID}).Err()
			if err != nil {
				http.Error(w, "Error adding friend", http.StatusInternalServerError)
				return
			}

			err = rdb.ZAdd(ctx, constants.FriendPrefix+request.RequesterID, &redis.Z{Score: float64(time.Now().Unix()), Member: strconv.Itoa(userID)}).Err()
			if err != nil {
				http.Error(w, "Error adding friend", http.StatusInternalServerError)
				return
			}

			// Arkadaşlık isteğini kaldır
			err = rdb.ZRem(ctx, friendRequestsKey, request.RequesterID).Err()
			if err != nil {
				http.Error(w, "Error removing friend request", http.StatusInternalServerError)
				return
			}
		} else if request.Status == "reject" {
			// Arkadaşlık isteğini kaldır
			err = rdb.ZRem(ctx, friendRequestsKey, request.RequesterID).Err()
			if err != nil {
				http.Error(w, "Error removing friend request", http.StatusInternalServerError)
				return
			}
		}

		// Yanıtı oluştur
		response := Response{
			Status: true,
			Result: "Friend request processed",
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			http.Error(w, "Error encoding response", http.StatusInternalServerError)
			return
		}
	}
}

func FriendListHandler(rdb *redis.Client, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Validate token and get user ID
		userID, err := authent.GetUserIDFromToken(rdb, ctx, r)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Get pagination parameters
		pageStr := r.URL.Query().Get("page")
		countStr := r.URL.Query().Get("count")

		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			http.Error(w, "Invalid page parameter", http.StatusBadRequest)
			return
		}

		count, err := strconv.Atoi(countStr)
		if err != nil || count < 1 {
			http.Error(w, "Invalid count parameter", http.StatusBadRequest)
			return
		}

		// Fetch friends list
		start := (page - 1) * count
		end := start + count - 1

		friends, err := rdb.ZRange(ctx, constants.FriendPrefix+strconv.Itoa(userID), int64(start), int64(end)).Result()
		if err != nil {
			http.Error(w, "Error retrieving friends list", http.StatusInternalServerError)
			return
		}

		var friendDetails []FriendDetails
		for _, friendID := range friends {
			userData, err := rdb.Get(ctx, constants.UserPrefix+friendID).Result()
			if err != nil {
				if err == redis.Nil {
					continue
				}
				http.Error(w, "Error retrieving user data", http.StatusInternalServerError)
				return
			}

			var userDetails UserDetails
			err = json.Unmarshal([]byte(userData), &userDetails)
			if err != nil {
				http.Error(w, "Error parsing user data", http.StatusInternalServerError)
				return
			}

			friendDetails = append(friendDetails, FriendDetails{
				UserID:   friendID,
				Username: userDetails.Username,
			})
		}

		response := Response{
			Status:  true,
			Result:  friendDetails,
			Message: "",
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			http.Error(w, "Error encoding response", http.StatusInternalServerError)
			return
		}
	}
}
