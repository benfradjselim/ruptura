// Shop app for the precursor benchmark. ROLE selects load, api or db.
// The db drifts through random episodes: real slowdowns push api memory past its
// limit, decoys look the same at first and fade, surges raise traffic only.
package main

import (
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var delayMS int64 = 5

func db() {
	go func() {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		for {
			time.Sleep(time.Duration(20+rng.Intn(30)) * time.Minute)
			peak, minutes := 600.0, 12+rng.Intn(8)
			kind := "failure"
			if rng.Float64() < 0.5 {
				peak, minutes, kind = 120+rng.Float64()*130, 12+rng.Intn(8), "decoy"
			}
			fmt.Println(time.Now().UTC().Format(time.RFC3339), "episode", kind, minutes)
			for i := 0; i <= minutes*6; i++ {
				f := float64(i) / float64(minutes*6)
				atomic.StoreInt64(&delayMS, 5+int64(peak*f*f))
				time.Sleep(10 * time.Second)
			}
			if kind == "decoy" {
				for i := 36; i >= 0; i-- {
					atomic.StoreInt64(&delayMS, 5+int64(peak*float64(i)/36))
					time.Sleep(10 * time.Second)
				}
			}
			time.Sleep(time.Minute)
			atomic.StoreInt64(&delayMS, 5)
		}
	}()
	http.HandleFunc("/q", func(w http.ResponseWriter, _ *http.Request) {
		d := atomic.LoadInt64(&delayMS)
		time.Sleep(time.Duration(float64(d)*(0.7+0.6*rand.Float64())) * time.Millisecond)
		w.Write(make([]byte, 2048))
	})
	http.ListenAndServe(":8080", nil)
}

func api() {
	client := &http.Client{Timeout: 30 * time.Second}
	backend := os.Getenv("DB_URL")
	pool := make(chan struct{}, 12)
	var mu sync.Mutex
	held := map[int][]byte{}
	var id int64
	http.HandleFunc("/buy", func(w http.ResponseWriter, _ *http.Request) {
		n := int(atomic.AddInt64(&id, 1))
		buf := make([]byte, 512<<10)
		for i := 0; i < len(buf); i += 4096 {
			buf[i] = 1
		}
		mu.Lock()
		held[n] = buf
		mu.Unlock()
		defer func() {
			mu.Lock()
			delete(held, n)
			mu.Unlock()
		}()
		pool <- struct{}{}
		defer func() { <-pool }()
		resp, err := client.Get(backend)
		if err != nil {
			http.Error(w, "db", http.StatusBadGateway)
			return
		}
		io.Copy(w, resp.Body)
		resp.Body.Close()
	})
	http.ListenAndServe(":8080", nil)
}

func load() {
	client := &http.Client{Timeout: 40 * time.Second}
	target := os.Getenv("API_URL")
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	surgeUntil := time.Time{}
	for t := 0; ; t++ {
		if rng.Float64() < 1.0/2400 {
			surgeUntil = time.Now().Add(time.Duration(5+rng.Intn(10)) * time.Minute)
		}
		rate := 30 + 8*math.Sin(float64(t)/600)
		if time.Now().Before(surgeUntil) {
			rate *= 2.5
		}
		for i := 0; i < int(rate); i++ {
			go func() {
				if resp, err := client.Get(target); err == nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
			}()
			time.Sleep(time.Second / time.Duration(rate+1))
		}
	}
}

func main() {
	switch os.Getenv("ROLE") {
	case "db":
		db()
	case "api":
		api()
	default:
		load()
	}
}
