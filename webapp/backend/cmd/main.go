package main

import (
	cache "backend/internal"
	"backend/internal/server"
	"log"
)

func main() {
	srv, dbConn, err := server.NewServer()
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}
	if dbConn != nil {
		defer dbConn.Close()
	}

	go cache.InitCache(dbConn)
	srv.Run()
}
