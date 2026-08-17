// Command task037-textdiff runs the text diff HTTP service. With the
// --smoke-test flag it executes the built-in self checks and exits.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"task037-textdiff/internal/httpapi"
	"task037-textdiff/internal/selfcheck"
)

func main() {
	smoke := flag.Bool("smoke-test", false, "run built-in self checks and exit")
	addr := flag.String("addr", ":8080", "listen address for the HTTP server")
	flag.Parse()

	if *smoke {
		if err := selfcheck.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "smoke test failed:", err)
			os.Exit(1)
		}
		fmt.Println("smoke test passed")
		return
	}

	mux := httpapi.New().Handler()
	srv := &http.Server{Addr: *addr, Handler: mux}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		log.Printf("textdiff listening on %s", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()
	<-stop
	if err := srv.Close(); err != nil {
		log.Printf("server close: %v", err)
	}
}
