package main

import (
	"context"
	"log"

	"github.com/V4T54L/goship/pkg/goship/db"
	"github.com/V4T54L/goship/pkg/goship/server"
	"github.com/V4T54L/watch-tower/internal/http/router"
	"github.com/V4T54L/watch-tower/pkg/config"
	"github.com/go-chi/chi/v5"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("error loading the config:", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	redisClient, err := db.ConnectToRedisDb(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatal("Error initalizing redis client: ", err)
	}

	postgresClient, err := db.ConnectToPostgresDb(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Error connecting to postgres:", err)
	}

	// TODO: remove this
	_, _ = redisClient, postgresClient

	server := server.NewChiServer()
	server.AddDefaultMiddleware()
	server.AddPermissiveCORS()
	server.AddDefaultRoutes()

	r, ok := server.GetRouter().(*chi.Mux)
	if !ok {
		log.Fatal("Error obtaining the router")
	}

	router.RegisterRoutes(r)

	err = server.Run("8000")
	if err != nil {
		log.Println("Error when running the server: ", err)
	}
}
