package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/kmradished/GOsapiens/internal/demodata"
	"github.com/kmradished/GOsapiens/internal/profileapi"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8081", "listen address")
	mode := flag.String("mode", "ok", "ok, missing, error, slow or large")
	slowDelay := flag.Duration("slow-delay", 2*time.Second, "delay in slow mode")
	largeBytes := flag.Int("large-bytes", 8192, "padding bytes in large mode")
	flag.Parse()

	if *slowDelay <= 0 || *largeBytes <= 0 {
		log.Fatal("slow-delay and large-bytes must be positive")
	}
	profileServer := profileapi.NewServer(demodata.Profiles(), *slowDelay, *largeBytes)
	if err := profileServer.SetMode(*mode); err != nil {
		log.Fatal(err)
	}
	server := &http.Server{
		Addr:              *addr,
		Handler:           profileServer.Handler(),
		ReadHeaderTimeout: 2 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	log.Printf("Profile API listening on %s (%s)", server.Addr, *mode)
	log.Fatal(server.ListenAndServe())
}
