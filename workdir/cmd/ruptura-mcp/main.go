// Command ruptura-mcp serves the read-only results of a Ruptura engine to AI
// assistants over the Model Context Protocol, on stdio or streamable HTTP.
package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/benfradjselim/ruptura/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		log.Fatalf("ruptura-mcp: %v", err)
	}
}

func run() error {
	engineURL := flag.String("engine", envOr("RUPTURA_URL", "http://ruptura:8080"), "Ruptura engine URL")
	listen := flag.String("listen", "", "serve streamable HTTP on this address, for example :8090; stdio when empty")
	flag.Parse()

	apiKey, err := secret("RUPTURA_API_KEY")
	if err != nil {
		return err
	}
	server := mcpserver.New(mcpserver.NewEngine(*engineURL, apiKey), version)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *listen == "" {
		return server.Run(ctx, &mcp.StdioTransport{})
	}
	token, err := secret("RUPTURA_MCP_TOKEN")
	if err != nil {
		return err
	}
	if token == "" {
		return errors.New("HTTP mode needs RUPTURA_MCP_TOKEN (or RUPTURA_MCP_TOKEN_FILE): clients send it as a bearer token")
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", requireToken(token, mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Printf("ruptura-mcp %s: streamable HTTP on %s/mcp, engine %s", version, *listen, mcpserver.Redact(*engineURL))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-drained
	return nil
}

// requireToken accepts only requests carrying the bearer token.
func requireToken(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "missing or wrong bearer token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// secret reads name from the environment, or from the file named by name_FILE.
func secret(name string) (string, error) {
	if path := os.Getenv(name + "_FILE"); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s_FILE: %w", name, err)
		}
		return strings.TrimSpace(string(raw)), nil
	}
	return strings.TrimSpace(os.Getenv(name)), nil
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
