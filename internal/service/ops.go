package service

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type serverReadiness struct {
	ID              string `json:"server"`
	Connected       bool   `json:"connected"`
	CompleteCatalog bool   `json:"complete_catalog"`
	Reason          string `json:"reason,omitempty"`
}

type ReadinessState struct {
	mu              sync.RWMutex
	configValid     bool
	writerLock      bool
	storageWritable bool
	routerReady     bool
	servers         map[string]serverReadiness
}

func NewReadinessState(configValid, writerLock, storageWritable, routerReady bool) *ReadinessState {
	return &ReadinessState{configValid: configValid, writerLock: writerLock, storageWritable: storageWritable, routerReady: routerReady, servers: make(map[string]serverReadiness)}
}

func (s *ReadinessState) SetLocal(configValid, writerLock, storageWritable, routerReady bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configValid, s.writerLock, s.storageWritable, s.routerReady = configValid, writerLock, storageWritable, routerReady
}

// Level reports the aggregate readiness level the operations endpoint would
// return: 0 unavailable, 1 degraded, 2 ready (plan §4.12).
func (s *ReadinessState) Level() int {
	level, _ := s.snapshot()
	return level
}

func (s *ReadinessState) SetServer(id string, connected, completeCatalog bool, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.servers[id] = serverReadiness{ID: id, Connected: connected, CompleteCatalog: completeCatalog, Reason: reason}
}

func (s *ReadinessState) snapshot() (int, []serverReadiness) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	servers := make([]serverReadiness, 0, len(s.servers))
	usable, degraded := 0, false
	for _, server := range s.servers {
		servers = append(servers, server)
		if server.Connected && server.CompleteCatalog {
			usable++
		} else {
			degraded = true
		}
	}
	sort.Slice(servers, func(i, j int) bool { return servers[i].ID < servers[j].ID })
	if !s.configValid || !s.writerLock || !s.storageWritable || !s.routerReady || usable == 0 {
		return 0, servers
	}
	if degraded || usable != len(servers) {
		return 1, servers
	}
	return 2, servers
}

func NewOperationsHandler(state *ReadinessState, metrics http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("live\n"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		stateValue, servers := state.snapshot()
		w.Header().Set("Content-Type", "application/json")
		if stateValue == 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		status := map[int]string{0: "unavailable", 1: "degraded", 2: "ready"}[stateValue]
		_ = json.NewEncoder(w).Encode(struct {
			Status  string            `json:"status"`
			Servers []serverReadiness `json:"servers"`
		}{Status: status, Servers: servers})
	})
	if metrics == nil {
		metrics = http.NotFoundHandler()
	}
	mux.Handle("/metrics", metrics)
	return mux
}

// RateLimit applies one bounded listener-wide token bucket to operational routes.
func RateLimit(next http.Handler, requestsPerSecond float64, burst int) http.Handler {
	if requestsPerSecond <= 0 || burst <= 0 {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "operations rate limiter is not configured", http.StatusServiceUnavailable)
		})
	}
	limiter := rate.NewLimiter(rate.Limit(requestsPerSecond), burst)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.AllowN(time.Now(), 1) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
