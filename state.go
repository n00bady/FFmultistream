package main

import (
	"context"
	"errors"
	"log"
	"sync"
)

var (
	errAlreadyRunning = errors.New("ffmpeg is already running")
	errNotRunning     = errors.New("ffmpeg is not running")
	errNoDestinations = errors.New("no destinations configured")
	errNoEnabled      = errors.New("no enabled destinations")
	errOriginNotFound = errors.New("origin not found")
	errDestNotFound   = errors.New("destination not found")
)

const originLogBuffer = 500

// OriginView is a read-only snapshot of an origin suitable for rendering.
type OriginView struct {
	ID           string
	URL          string
	Destinations []Destination
	Running      bool
}

func newAppState(cfg Config) *AppState {
	s := &AppState{
		origins:  make(map[string]*OriginRuntime, len(cfg.Origins)),
		username: cfg.Username,
		password: cfg.Password,
	}
	for _, o := range cfg.Origins {
		s.origins[o.ID] = &OriginRuntime{
			origin: cloneOrigin(o),
			logs:   newRingBuffer(originLogBuffer),
		}
		s.order = append(s.order, o.ID)
	}
	return s
}

func (s *AppState) Credentials() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.username, s.password
}

// configForSave returns a Config suitable for SaveConfig. The caller must not
// hold any origin runtime locks.
func (s *AppState) configForSave() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := Config{
		Username: s.username,
		Password: s.password,
	}
	for _, id := range s.order {
		rt, ok := s.origins[id]
		if !ok {
			continue
		}
		rt.mu.Lock()
		cfg.Origins = append(cfg.Origins, cloneOrigin(rt.origin))
		rt.mu.Unlock()
	}
	return cfg
}

// Snapshot returns view objects for every origin in declared order.
func (s *AppState) Snapshot() []OriginView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]OriginView, 0, len(s.order))
	for _, id := range s.order {
		rt, ok := s.origins[id]
		if !ok {
			continue
		}
		rt.mu.Lock()
		out = append(out, OriginView{
			ID:           rt.origin.ID,
			URL:          rt.origin.URL,
			Destinations: append([]Destination(nil), rt.origin.Destinations...),
			Running:      rt.running,
		})
		rt.mu.Unlock()
	}
	return out
}

func (s *AppState) OriginView(oid string) (OriginView, bool) {
	rt, ok := s.lookup(oid)
	if !ok {
		return OriginView{}, false
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return OriginView{
		ID:           rt.origin.ID,
		URL:          rt.origin.URL,
		Destinations: append([]Destination(nil), rt.origin.Destinations...),
		Running:      rt.running,
	}, true
}

func (s *AppState) lookup(oid string) (*OriginRuntime, bool) {
	s.mu.RLock()
	rt, ok := s.origins[oid]
	s.mu.RUnlock()
	return rt, ok
}

func (s *AppState) Logs(oid string) ([]string, bool) {
	rt, ok := s.lookup(oid)
	if !ok {
		return nil, false
	}
	return rt.logs.Snapshot(), true
}

// AddOrigin creates a new origin with the given URL. Returns the new ID.
func (s *AppState) AddOrigin(rtmpURL string) string {
	id := newOriginID()
	s.mu.Lock()
	for _, exists := s.origins[id]; exists; _, exists = s.origins[id] {
		id = newOriginID()
	}
	s.origins[id] = &OriginRuntime{
		origin: Origin{ID: id, URL: rtmpURL},
		logs:   newRingBuffer(originLogBuffer),
	}
	s.order = append(s.order, id)
	s.mu.Unlock()
	return id
}

func (s *AppState) UpdateOrigin(oid, rtmpURL string) bool {
	rt, ok := s.lookup(oid)
	if !ok {
		return false
	}
	rt.mu.Lock()
	rt.origin.URL = rtmpURL
	rt.mu.Unlock()
	return true
}

// DeleteOrigin stops the origin if running, then removes it.
func (s *AppState) DeleteOrigin(oid string) bool {
	rt, ok := s.lookup(oid)
	if !ok {
		return false
	}
	if done := rt.stop(); done != nil {
		<-done
	}
	s.mu.Lock()
	delete(s.origins, oid)
	for i, id := range s.order {
		if id == oid {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
	return true
}

func (s *AppState) AddDestination(oid, rtmp, key string) bool {
	rt, ok := s.lookup(oid)
	if !ok {
		return false
	}
	rt.mu.Lock()
	rt.origin.Destinations = append(rt.origin.Destinations, Destination{
		RTMP: rtmp, Key: key, Enabled: true,
	})
	rt.mu.Unlock()
	return true
}

func (s *AppState) UpdateDestination(oid string, i int, rtmp, key string) bool {
	rt, ok := s.lookup(oid)
	if !ok {
		return false
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if i < 0 || i >= len(rt.origin.Destinations) {
		return false
	}
	rt.origin.Destinations[i].RTMP = rtmp
	rt.origin.Destinations[i].Key = key
	return true
}

func (s *AppState) DeleteDestination(oid string, i int) bool {
	rt, ok := s.lookup(oid)
	if !ok {
		return false
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if i < 0 || i >= len(rt.origin.Destinations) {
		return false
	}
	rt.origin.Destinations = append(rt.origin.Destinations[:i], rt.origin.Destinations[i+1:]...)
	return true
}

// ToggleDestination flips a destination's enabled flag and reports whether the
// origin's ffmpeg was running before the toggle.
func (s *AppState) ToggleDestination(oid string, i int) (nowEnabled, wasRunning, ok bool) {
	rt, found := s.lookup(oid)
	if !found {
		return false, false, false
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if i < 0 || i >= len(rt.origin.Destinations) {
		return false, false, false
	}
	rt.origin.Destinations[i].Enabled = !rt.origin.Destinations[i].Enabled
	return rt.origin.Destinations[i].Enabled, rt.running, true
}

// OriginRunning returns whether ffmpeg is currently running for the origin.
func (s *AppState) OriginRunning(oid string) bool {
	rt, ok := s.lookup(oid)
	if !ok {
		return false
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.running
}

// BeginRun reserves the origin's ffmpeg slot. Returns the build args and a
// context that is cancelled when Stop is called.
func (s *AppState) BeginRun(oid string) (context.Context, []string, *ringBuffer, error) {
	rt, ok := s.lookup(oid)
	if !ok {
		return nil, nil, nil, errOriginNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.running {
		return nil, nil, nil, errAlreadyRunning
	}
	if len(rt.origin.Destinations) == 0 {
		return nil, nil, nil, errNoDestinations
	}
	if enabledCount(rt.origin) == 0 {
		return nil, nil, nil, errNoEnabled
	}
	ctx, cancel := context.WithCancel(context.Background())
	rt.cancel = cancel
	rt.running = true
	rt.done = make(chan struct{})
	return ctx, buildFFmpegArgs(rt.origin), rt.logs, nil
}

// CanStart checks whether the origin is in a state where a run can begin.
func (s *AppState) CanStart(oid string) error {
	rt, ok := s.lookup(oid)
	if !ok {
		return errOriginNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.running {
		return errAlreadyRunning
	}
	if len(rt.origin.Destinations) == 0 {
		return errNoDestinations
	}
	if enabledCount(rt.origin) == 0 {
		return errNoEnabled
	}
	return nil
}

func (s *AppState) MarkStopped(oid string) {
	rt, ok := s.lookup(oid)
	if !ok {
		return
	}
	rt.markStopped()
}

func (s *AppState) StopOrigin(oid string) <-chan struct{} {
	rt, ok := s.lookup(oid)
	if !ok {
		return nil
	}
	return rt.stop()
}

// Stop tears down every running origin in parallel and waits for them.
func (s *AppState) Stop() {
	s.mu.RLock()
	dones := make([]<-chan struct{}, 0, len(s.origins))
	for _, rt := range s.origins {
		if d := rt.stop(); d != nil {
			dones = append(dones, d)
		}
	}
	s.mu.RUnlock()
	var wg sync.WaitGroup
	for _, d := range dones {
		wg.Add(1)
		go func(c <-chan struct{}) {
			defer wg.Done()
			<-c
		}(d)
	}
	wg.Wait()
}

func (rt *OriginRuntime) stop() <-chan struct{} {
	rt.mu.Lock()
	cancel := rt.cancel
	done := rt.done
	id := rt.origin.ID
	rt.mu.Unlock()
	if cancel != nil {
		cancel()
		log.Printf("FFmpeg stop requested for origin %s.", id)
	}
	return done
}

func (rt *OriginRuntime) markStopped() {
	rt.mu.Lock()
	rt.running = false
	rt.cancel = nil
	done := rt.done
	rt.done = nil
	rt.mu.Unlock()
	if done != nil {
		close(done)
	}
}

func cloneOrigin(o Origin) Origin {
	return Origin{
		ID:           o.ID,
		URL:          o.URL,
		Destinations: append([]Destination(nil), o.Destinations...),
	}
}
