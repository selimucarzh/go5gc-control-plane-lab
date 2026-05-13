package main

import (
	"flag"
	"log"
	"os"

	"go5gc-control-plane-lab/internal/cpstub"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	server := cpstub.NewServer(logger)

	logger.Printf("cp-stub listening on %s", *addr)
	if err := server.ListenAndServe(*addr); err != nil {
		log.Fatalf("run cp-stub: %v", err)
	}
}
