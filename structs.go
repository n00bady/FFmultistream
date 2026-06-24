package main

import (
	"context"
	"sync"
)

type Destination struct {
	RTMP    string `toml:"RTMP"`
	Key     string `toml:"Key"`
	Enabled bool   `toml:"Enabled"`
}

type Origin struct {
	ID           string        `toml:"ID"`
	URL          string        `toml:"URL"`
	Destinations []Destination `toml:"Destinations"`
}

type Config struct {
	Origins  []Origin `toml:"Origins"`
	Username string   `toml:"Username"`
	Password string   `toml:"Password"`

	// Legacy fields (read-only) used for one-shot migration from the
	// previous single-origin schema.
	LegacyOrigin       string   `toml:"Origin,omitempty"`
	LegacyDestinations []string `toml:"Destinations,omitempty"`
	LegacyKeys         []string `toml:"Keys,omitempty"`
	LegacyEnabled      []bool   `toml:"Enabled,omitempty"`
}

type OriginRuntime struct {
	mu      sync.Mutex
	origin  Origin
	cancel  context.CancelFunc
	running bool
	done    chan struct{}
	logs    *ringBuffer
}

type AppState struct {
	mu       sync.RWMutex
	origins  map[string]*OriginRuntime
	order    []string
	username string
	password string
}

type ringBuffer struct {
	mu    sync.Mutex
	lines []string
	head  int
	size  int
}

func newRingBuffer(max int) *ringBuffer {
	return &ringBuffer{lines: make([]string, max)}
}

func (r *ringBuffer) Append(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines[r.head] = line
	r.head = (r.head + 1) % len(r.lines)
	if r.size < len(r.lines) {
		r.size++
	}
}

func (r *ringBuffer) Snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, r.size)
	start := r.head - r.size
	if start < 0 {
		start += len(r.lines)
	}
	for i := 0; i < r.size; i++ {
		out[i] = r.lines[(start+i)%len(r.lines)]
	}
	return out
}
