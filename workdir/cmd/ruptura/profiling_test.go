package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestStartProfiling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// A non loopback address is refused: nothing listens.
	startProfiling(ctx, "0.0.0.0:0")
	startProfiling(ctx, "")

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	startProfiling(ctx, addr)
	var resp *http.Response
	for i := 0; i < 50; i++ {
		if resp, err = http.Get("http://" + addr + "/debug/pprof/heap?debug=1"); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || len(body) == 0 {
		t.Fatalf("heap profile: %d, %d bytes", resp.StatusCode, len(body))
	}
}
