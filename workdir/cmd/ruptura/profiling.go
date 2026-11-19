package main

import (
	"context"
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/benfradjselim/ruptura/pkg/logger"
)

// profilingEnv names the address of the opt-in Go profiling endpoint, such as
// 127.0.0.1:6060. It is off when unset. A loopback address keeps it inside the
// pod; reach it with oc port-forward.
const profilingEnv = "RUPTURA_PPROF_ADDR"

// startProfiling serves the Go profiles on addr until ctx ends. Only loopback
// addresses are accepted, because the profiles are not authenticated.
func startProfiling(ctx context.Context, addr string) {
	if addr == "" {
		return
	}
	host, _, err := net.SplitHostPort(addr)
	if ip := net.ParseIP(host); err != nil || ip == nil || !ip.IsLoopback() {
		logger.Default.Warn("profiling disabled: address must be loopback", "addr", addr)
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Default.Warn("profiling stopped", "err", err)
		}
	}()
	logger.Default.Info("profiling enabled", "addr", addr)
}
