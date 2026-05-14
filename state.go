package main

import (
	"context"
	"errors"
	"log"
)

var (
	errAlreadyRunning = errors.New("ffmpeg is already running")
	errNoDestinations = errors.New("no destinations configured")
	errNoEnabled      = errors.New("no enabled destinations")
)

func (s *AppState) Snapshot() (Config, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneConfig(s.config), s.running
}

func (s *AppState) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *AppState) AddDestination(rtmp, key string) Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Destinations = append(s.config.Destinations, rtmp)
	s.config.Keys = append(s.config.Keys, key)
	s.config.Enabled = append(s.config.Enabled, true)
	return cloneConfig(s.config)
}

func (s *AppState) UpdateDestination(i int, rtmp, key string) (Config, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i < 0 || i >= len(s.config.Destinations) {
		return Config{}, false
	}
	s.config.Destinations[i] = rtmp
	s.config.Keys[i] = key
	return cloneConfig(s.config), true
}

func (s *AppState) DeleteDestination(i int) (Config, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i < 0 || i >= len(s.config.Destinations) {
		return Config{}, false
	}
	s.config.Destinations = append(s.config.Destinations[:i], s.config.Destinations[i+1:]...)
	s.config.Keys = append(s.config.Keys[:i], s.config.Keys[i+1:]...)
	s.config.Enabled = append(s.config.Enabled[:i], s.config.Enabled[i+1:]...)
	return cloneConfig(s.config), true
}

func (s *AppState) ToggleDestination(i int) (cfg Config, nowEnabled, wasRunning, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i < 0 || i >= len(s.config.Destinations) {
		return Config{}, false, false, false
	}
	if len(s.config.Enabled) != len(s.config.Destinations) {
		normalizeEnabled(&s.config)
	}
	s.config.Enabled[i] = !s.config.Enabled[i]
	return cloneConfig(s.config), s.config.Enabled[i], s.running, true
}

func (s *AppState) Credentials() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config.Username, s.config.Password
}

func (s *AppState) SetOrigin(origin string) Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Origin = origin
	return cloneConfig(s.config)
}

func (s *AppState) CanStart() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return errAlreadyRunning
	}
	if len(s.config.Destinations) == 0 {
		return errNoDestinations
	}
	if enabledCount(s.config) == 0 {
		return errNoEnabled
	}
	return nil
}

func (s *AppState) BeginRun() (context.Context, []string, chan struct{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil, nil, nil, errAlreadyRunning
	}
	if enabledCount(s.config) == 0 {
		return nil, nil, nil, errNoEnabled
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true
	s.done = make(chan struct{})
	return ctx, buildFFmpegArgs(s.config), s.done, nil
}

func (s *AppState) MarkStopped() {
	s.mu.Lock()
	s.running = false
	s.cancel = nil
	done := s.done
	s.done = nil
	s.mu.Unlock()
	if done != nil {
		close(done)
	}
}

func (s *AppState) Stop() <-chan struct{} {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		log.Println("FFmpeg process stopped.")
	}
	return done
}

func cloneConfig(c Config) Config {
	return Config{
		Origin:       c.Origin,
		Destinations: append([]string(nil), c.Destinations...),
		Keys:         append([]string(nil), c.Keys...),
		Enabled:      append([]bool(nil), c.Enabled...),
	}
}
