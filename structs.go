package main

import (
	"context"
	"sync"
)

type Config struct {
	Origin       string   `toml:"Origin"`
	Destinations []string `toml:"Destinations"`
	Keys         []string `toml:"Keys"`
	Enabled      []bool   `toml:"Enabled"`
}

type AppState struct {
	mu      sync.Mutex
	config  Config
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	done    chan struct{}
	logs    *ringBuffer
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
