package main

import (
	"log"
	"net/http"
	"os"

	"go5gc-control-plane-lab/internal/smf"
)

func main() {
	amfBaseURL := os.Getenv("AMF_BASE_URL")
	if amfBaseURL == "" {
		amfBaseURL = "http://127.0.0.1:8081"
	}

	amfClient := smf.NewHTTPAMFClient(amfBaseURL, nil)
	server := smf.NewServer("smf-001", amfClient)
	addr := ":8082"

	log.Printf("smf listening on %s using amf %s", addr, amfBaseURL)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("run smf: %v", err)
	}
}
