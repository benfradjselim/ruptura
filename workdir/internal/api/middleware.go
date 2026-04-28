package api

import (
    "net/http"
    "strings"
)

func (h *Handlers) authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if h.apiKey == "" { next.ServeHTTP(w, r); return }
        auth := r.Header.Get("Authorization")
        if !strings.HasPrefix(auth, "Bearer ") || auth[7:] != h.apiKey {
            writeError(w, http.StatusUnauthorized, "unauthorized")
            return
        }
        next.ServeHTTP(w, r)
    })
}

func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        next.ServeHTTP(w, r)
    })
}
