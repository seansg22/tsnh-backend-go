package main

import (
	"log"
	"net/http"

	handler "tsnh-backend-go/api"
)

func main() {
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", http.HandlerFunc(handler.Handler)))
}
