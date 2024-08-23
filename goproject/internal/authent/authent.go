package authent

import (
	"context"
	"fmt"
	"masomointern/internal/constants"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

func GenerateToken(rdb *redis.Client, ctx context.Context, userID int) (string, error) {
	token := uuid.New().String()
	key := fmt.Sprintf(constants.TokenPrefix+"%s", token)
	err := rdb.Set(ctx, key, userID, 24*time.Hour).Err() // Token'ı 24 saat geçerli yap
	if err != nil {
		return "", err
	}
	return token, nil
}

func GetUserIDFromToken(rdb *redis.Client, ctx context.Context, r *http.Request) (int, error) {
	token := r.Header.Get("Authorization")
	if token == "" {
		return 0, fmt.Errorf("Authorization token is missing")
	}

	// Eğer "Bearer" prefix varsa, onu çıkarın
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	key := fmt.Sprintf(constants.TokenPrefix+"%s", token)
	userIDStr, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, fmt.Errorf("Invalid or expired token")
		}
		return 0, err
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		return 0, fmt.Errorf("Invalid user ID in token")
	}

	return userID, nil
}

func TestGenerateAndGetToken(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()

	// Token üret
	userID := 123
	token, err := GenerateToken(rdb, ctx, userID)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Token'ı bir HTTP başlığına ekle
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	// Token'dan kullanıcı ID'sini al
	retrievedUserID, err := GetUserIDFromToken(rdb, ctx, req)
	if err != nil {
		t.Fatalf("Failed to get user ID from token: %v", err)
	}

	if retrievedUserID != userID {
		t.Fatalf("Expected user ID %d, got %d", userID, retrievedUserID)
	}
}
