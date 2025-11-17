package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	port := flag.Int("port", 9001, "Port to listen on")
	name := flag.String("name", "Backend", "Backend name")
	delay := flag.Duration("delay", 0, "Artificial delay for responses")
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if *delay > 0 {
			time.Sleep(*delay)
		}

		log.Printf("[%s] Request from %s: %s %s", *name, r.RemoteAddr, r.Method, r.URL.Path)

		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Backend", *name)
		fmt.Fprintf(w, "%s (port %d)\n", *name, *port)
		fmt.Fprintf(w, "Time: %s\n", time.Now().Format(time.RFC3339))
		fmt.Fprintf(w, "Request: %s %s\n", r.Method, r.URL.Path)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK\n")
	})

	http.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		fmt.Fprintf(w, "Slow response from %s\n", *name)
	})

	log.Printf("Starting %s on %s", *name, addr)
	log.Printf("Endpoints: /, /health, /slow")
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
