package main

import (
	"log"
	"net/http"

	"go5gc-control-plane-lab/internal/amf"
)

func main() {
	server := amf.NewServer("amf-001")
	addr := ":8081"

	log.Printf("amf listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("run amf: %v", err)
	}
}
