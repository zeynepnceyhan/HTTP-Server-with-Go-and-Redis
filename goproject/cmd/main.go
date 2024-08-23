package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"goproject/internal/friendship"
	"goproject/internal/match"
	"goproject/internal/middleware"
	"goproject/internal/simulation"
	"goproject/internal/user"

	"github.com/go-redis/redis/v8"
)

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()

	http.HandleFunc("/register", middleware.AuthMiddleware(rdb, ctx, "RegisterHandler", user.RegisterHandler(rdb, ctx)))
	http.HandleFunc("/login", middleware.AuthMiddleware(rdb, ctx, "LoginHandler", user.LoginHandler(rdb, ctx)))
	http.HandleFunc("/update", middleware.AuthMiddleware(rdb, ctx, "UpdateHandler", user.UpdateInfoHandler(rdb, ctx)))
	http.HandleFunc("/matchresult", middleware.AuthMiddleware(rdb, ctx, "MatchResultHandler", match.MatchResultHandler(rdb, ctx)))
	http.HandleFunc("/leaderboard", middleware.AuthMiddleware(rdb, ctx, "LeaderboardHandler", match.LeaderboardHandler(rdb, ctx)))
	http.HandleFunc("/userdetails", middleware.AuthMiddleware(rdb, ctx, "UserDetailsHandler", user.UserDetailsHandler(rdb, ctx)))
	http.HandleFunc("/simulation", middleware.AuthMiddleware(rdb, ctx, "SimulationHandler", simulation.SimulationHandler(rdb, ctx)))
	http.HandleFunc("/friendship/search", friendship.UserSearchHandler(rdb, ctx))
	http.HandleFunc("/friendship/friendrequest", friendship.FriendRequestHandler(rdb, ctx))
	http.HandleFunc("/friendship/friendrequestlist", friendship.FriendRequestListHandler(rdb, ctx))
	http.HandleFunc("/friendship/respondrequest", friendship.AcceptRejectFriendRequestHandler(rdb, ctx))
	http.HandleFunc("/friendship/friendlist", friendship.FriendListHandler(rdb, ctx))

	// Start the HTTP server
	server := &http.Server{
		Addr:         ":8080",
		Handler:      nil, // `http.DefaultServeMux` kullanılacak
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Println("Starting server on :8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
