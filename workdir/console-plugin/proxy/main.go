// Command ruptura-plugin-proxy sits between the console plugin's nginx and
// the Ruptura engine. It receives the console user's token, forwards the read
// endpoints of the engine with the API key, and keeps only the namespaces the
// user may see, as the API server decides. It listens on localhost only: nginx
// is the only caller.
package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultListen     = "127.0.0.1:9444"
	defaultBackendURL = "http://ruptura:8080"
	defaultKeyFile    = "/var/run/ruptura/api-key/api-key"
	serviceCAFile     = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	readTimeout       = 30 * time.Second
	accessCacheTTL    = 30 * time.Second
)

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func main() {
	listen := env("RUPTURA_PROXY_LISTEN", defaultListen)
	backend := env("RUPTURA_BACKEND_URL", defaultBackendURL)
	keyFile := env("RUPTURA_API_KEY_FILE", defaultKeyFile)

	raw, err := os.ReadFile(keyFile)
	if err != nil {
		log.Fatalf("ruptura-plugin-proxy: API key file: %v", err)
	}
	key := strings.TrimSpace(string(raw))
	if key == "" {
		log.Fatalf("ruptura-plugin-proxy: API key file %s is empty", keyFile)
	}

	apiServer := "https://" + env("KUBERNETES_SERVICE_HOST", "kubernetes.default.svc") + ":" + env("KUBERNETES_SERVICE_PORT", "443")
	checker, err := newAccessChecker(apiServer, serviceCAFile, readTimeout)
	if err != nil {
		log.Fatalf("ruptura-plugin-proxy: API server client: %v", err)
	}

	srv := &http.Server{
		Addr:              listen,
		Handler:           newHandler(backend, key, newCachedChecker(checker, accessCacheTTL), readTimeout),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("ruptura-plugin-proxy: listening on %s, engine %s, access decided by %s", listen, backend, apiServer)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("ruptura-plugin-proxy: %v", err)
	}
}
