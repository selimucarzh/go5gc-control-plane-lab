package main

import (
	"log"
	"net/http"

	"go5gc-control-plane-lab/internal/smf"
)

func main() {
	amfClient := smf.NewHTTPAMFClient("http://127.0.0.1:8081", nil)
	server := smf.NewServer("smf-001", amfClient)
	addr := ":8082"

	log.Printf("smf listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("run smf: %v", err)
	}
}
