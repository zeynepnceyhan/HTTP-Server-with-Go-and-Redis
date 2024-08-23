package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"masomointern/internal/authent"

	"github.com/go-redis/redis/v8"
)

// JSON formatında yanıt yapısı
type Response struct {
	Status  bool        `json:"status"`
	Result  interface{} `json:"result"`
	Message string      `json:"message"`
}

// İzin verilen metotları ve handler'ları bir haritada tanımlıyoruz
var allowedMethods = map[string]string{
	"RegisterHandler":    http.MethodPost,
	"LoginHandler":       http.MethodPost,
	"UpdateHandler":      http.MethodPost,
	"MatchResultHandler": http.MethodPost,
	"LeaderboardHandler": http.MethodGet,
	"UserDetailsHandler": http.MethodGet,
	"SimulationHandler":  http.MethodGet,
}

func AuthMiddleware(rdb *redis.Client, ctx context.Context, handlerName string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// HTTP metodunu kontrol ediyoruz
		if allowedMethod, ok := allowedMethods[handlerName]; ok {
			if r.Method != allowedMethod {
				// JSON formatında hata yanıtı döndür
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusMethodNotAllowed)
				json.NewEncoder(w).Encode(Response{Status: false, Result: nil, Message: "Method not allowed"})
				return
			}
		} else {
			// Handler adı mevcut değilse bir hata döndür
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(Response{Status: false, Result: nil, Message: "Handler not found"})
			return
		}

		// Register ve Login dışındaki isteklerde token doğrulaması yapıyoruz
		if handlerName != "RegisterHandler" && handlerName != "LoginHandler" {
			userID, err := authent.GetUserIDFromToken(rdb, ctx, r)
			if err != nil {
				// JSON formatında hata yanıtı döndür
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(Response{Status: false, Result: nil, Message: "Unauthorized: " + err.Error()})
				return
			}

			// Kullanıcı ID'sini request context'e ekliyoruz
			ctx = context.WithValue(r.Context(), "userID", userID)
			r = r.WithContext(ctx)
		}

		// İstek başarılıysa handler'ı çağır
		next.ServeHTTP(w, r)
	}
}
