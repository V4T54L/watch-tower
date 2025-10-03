package main

import (
	"log"

	"github.com/V4T54L/goship/pkg/goship/server"
	"github.com/V4T54L/watch-tower/internal/http/router"
	"github.com/go-chi/chi/v5"
)

func main() {
	server := server.NewChiServer()
	server.AddDefaultMiddleware()
	server.AddPermissiveCORS()
	server.AddDefaultRoutes()

	r, ok := server.GetRouter().(*chi.Mux)
	if !ok {
		log.Fatal("Error obtaining the router")
	}

	router.RegisterRoutes(r)

	err := server.Run("8000")
	if err != nil {
		log.Println("Error when running the server: ", err)
	}
}
