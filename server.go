package main

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Server struct {
	state *AppState
	tpl   *templates
}

func newServer(state *AppState, tpl *templates) *Server {
	return &Server{state: state, tpl: tpl}
}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /destinations", s.handleAddDestination)
	mux.HandleFunc("GET /destinations/{i}/edit", s.handleEditDestination)
	mux.HandleFunc("POST /destinations/{i}", s.handleUpdateDestination)
	mux.HandleFunc("POST /destinations/{i}/delete", s.handleDeleteDestination)
	mux.HandleFunc("GET /settings", s.handleSettings)
	mux.HandleFunc("POST /settings", s.handleUpdateSettings)
	mux.HandleFunc("POST /stream/start", s.handleStartStream)
	mux.HandleFunc("POST /stream/stop", s.handleStopStream)
	mux.HandleFunc("GET /logs", s.handleLogs)
	return mux
}

var flashMessages = map[string]string{
	"added":   "Destination added.",
	"saved":   "Saved.",
	"deleted": "Destination deleted.",
	"started": "Stream started.",
	"stopped": "Stream stopped.",
}

var flashErrors = map[string]string{
	"invalid_rtmp":    "RTMP URL is not valid.",
	"invalid_key":     "Stream key cannot be empty.",
	"not_found":       "Destination not found.",
	"already_running": "Stream is already running.",
	"not_running":     "Stream is not running.",
	"no_destinations": "No destinations configured.",
	"save_failed":     "Failed to save configuration.",
	"start_failed":    "Failed to start ffmpeg.",
	"forbidden":       "Request rejected (cross-origin).",
}

type baseView struct {
	Config   Config
	Running  bool
	Flash    string
	FlashErr string
}

type indexView struct {
	baseView
	MaskedKeys []string
}

type editView struct {
	baseView
	Index int
	RTMP  string
	Key   string
}

type logsView struct {
	baseView
	Logs []string
}

func (s *Server) snapshot(r *http.Request) baseView {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	cfg := s.state.config
	cfgCopy := Config{
		Origin:       cfg.Origin,
		Destinations: append([]string(nil), cfg.Destinations...),
		Keys:         append([]string(nil), cfg.Keys...),
	}
	return baseView{
		Config:   cfgCopy,
		Running:  s.state.running,
		Flash:    flashMessages[r.URL.Query().Get("msg")],
		FlashErr: flashErrors[r.URL.Query().Get("err")],
	}
}

func (s *Server) render(w http.ResponseWriter, t *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("template render error: %v", err)
	}
}

func (s *Server) checkOrigin(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return u.Host == r.Host
	}
	if ref := r.Header.Get("Referer"); ref != "" {
		u, err := url.Parse(ref)
		if err != nil {
			return false
		}
		return u.Host == r.Host
	}
	return false
}

func redirectFlash(w http.ResponseWriter, r *http.Request, path, kind, code string) {
	target := path
	if code != "" {
		target += "?" + kind + "=" + url.QueryEscape(code)
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	base := s.snapshot(r)
	masked := make([]string, len(base.Config.Keys))
	for i, k := range base.Config.Keys {
		masked[i] = maskKey(k)
	}
	s.render(w, s.tpl.index, indexView{baseView: base, MaskedKeys: masked})
}

func (s *Server) handleAddDestination(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(r) {
		redirectFlash(w, r, "/", "err", "forbidden")
		return
	}
	if err := r.ParseForm(); err != nil {
		redirectFlash(w, r, "/", "err", "invalid_rtmp")
		return
	}
	rtmp := strings.TrimSpace(r.PostFormValue("rtmp"))
	key := strings.TrimSpace(r.PostFormValue("key"))
	if !IsvalidRTMP(rtmp) {
		redirectFlash(w, r, "/", "err", "invalid_rtmp")
		return
	}
	if !IsvalidKEY(key) {
		redirectFlash(w, r, "/", "err", "invalid_key")
		return
	}

	s.state.mu.Lock()
	s.state.config.Destinations = append(s.state.config.Destinations, rtmp)
	s.state.config.Keys = append(s.state.config.Keys, key)
	cfg := s.state.config
	s.state.mu.Unlock()

	if err := SaveConfig(cfg); err != nil {
		log.Printf("SaveConfig failed: %v", err)
		redirectFlash(w, r, "/", "err", "save_failed")
		return
	}
	redirectFlash(w, r, "/", "msg", "added")
}

func (s *Server) handleEditDestination(w http.ResponseWriter, r *http.Request) {
	idx, ok := s.parseIndex(w, r)
	if !ok {
		return
	}
	base := s.snapshot(r)
	if idx < 0 || idx >= len(base.Config.Destinations) {
		redirectFlash(w, r, "/", "err", "not_found")
		return
	}
	s.render(w, s.tpl.edit, editView{
		baseView: base,
		Index:    idx,
		RTMP:     base.Config.Destinations[idx],
		Key:      base.Config.Keys[idx],
	})
}

func (s *Server) handleUpdateDestination(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(r) {
		redirectFlash(w, r, "/", "err", "forbidden")
		return
	}
	idx, ok := s.parseIndex(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		redirectFlash(w, r, "/", "err", "invalid_rtmp")
		return
	}
	rtmp := strings.TrimSpace(r.PostFormValue("rtmp"))
	key := strings.TrimSpace(r.PostFormValue("key"))
	if !IsvalidRTMP(rtmp) {
		redirectFlash(w, r, "/", "err", "invalid_rtmp")
		return
	}
	if !IsvalidKEY(key) {
		redirectFlash(w, r, "/", "err", "invalid_key")
		return
	}

	s.state.mu.Lock()
	if idx < 0 || idx >= len(s.state.config.Destinations) {
		s.state.mu.Unlock()
		redirectFlash(w, r, "/", "err", "not_found")
		return
	}
	s.state.config.Destinations[idx] = rtmp
	s.state.config.Keys[idx] = key
	cfg := s.state.config
	s.state.mu.Unlock()

	if err := SaveConfig(cfg); err != nil {
		log.Printf("SaveConfig failed: %v", err)
		redirectFlash(w, r, "/", "err", "save_failed")
		return
	}
	redirectFlash(w, r, "/", "msg", "saved")
}

func (s *Server) handleDeleteDestination(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(r) {
		redirectFlash(w, r, "/", "err", "forbidden")
		return
	}
	idx, ok := s.parseIndex(w, r)
	if !ok {
		return
	}

	s.state.mu.Lock()
	if idx < 0 || idx >= len(s.state.config.Destinations) {
		s.state.mu.Unlock()
		redirectFlash(w, r, "/", "err", "not_found")
		return
	}
	s.state.config.Destinations = append(s.state.config.Destinations[:idx], s.state.config.Destinations[idx+1:]...)
	s.state.config.Keys = append(s.state.config.Keys[:idx], s.state.config.Keys[idx+1:]...)
	cfg := s.state.config
	s.state.mu.Unlock()

	if err := SaveConfig(cfg); err != nil {
		log.Printf("SaveConfig failed: %v", err)
		redirectFlash(w, r, "/", "err", "save_failed")
		return
	}
	redirectFlash(w, r, "/", "msg", "deleted")
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	s.render(w, s.tpl.settings, s.snapshot(r))
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(r) {
		redirectFlash(w, r, "/settings", "err", "forbidden")
		return
	}
	if err := r.ParseForm(); err != nil {
		redirectFlash(w, r, "/settings", "err", "invalid_rtmp")
		return
	}
	origin := strings.TrimSpace(r.PostFormValue("origin"))
	if !IsvalidRTMP(origin) {
		redirectFlash(w, r, "/settings", "err", "invalid_rtmp")
		return
	}

	s.state.mu.Lock()
	s.state.config.Origin = origin
	cfg := s.state.config
	s.state.mu.Unlock()

	if err := SaveConfig(cfg); err != nil {
		log.Printf("SaveConfig failed: %v", err)
		redirectFlash(w, r, "/settings", "err", "save_failed")
		return
	}
	redirectFlash(w, r, "/settings", "msg", "saved")
}

func (s *Server) handleStartStream(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(r) {
		redirectFlash(w, r, "/", "err", "forbidden")
		return
	}

	s.state.mu.Lock()
	if s.state.running {
		s.state.mu.Unlock()
		redirectFlash(w, r, "/", "err", "already_running")
		return
	}
	if len(s.state.config.Destinations) == 0 {
		s.state.mu.Unlock()
		redirectFlash(w, r, "/", "err", "no_destinations")
		return
	}
	s.state.mu.Unlock()

	go func() {
		if err := startFFmpeg(s.state); err != nil {
			log.Printf("ffmpeg error: %v", err)
		}
	}()
	redirectFlash(w, r, "/", "msg", "started")
}

func (s *Server) handleStopStream(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(r) {
		redirectFlash(w, r, "/", "err", "forbidden")
		return
	}
	s.state.mu.Lock()
	running := s.state.running
	s.state.mu.Unlock()
	if !running {
		redirectFlash(w, r, "/", "err", "not_running")
		return
	}
	stopFFmpeg(s.state)
	redirectFlash(w, r, "/", "msg", "stopped")
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	base := s.snapshot(r)
	lines := s.state.logs.Snapshot()
	s.render(w, s.tpl.logs, logsView{baseView: base, Logs: lines})
}

func (s *Server) parseIndex(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.PathValue("i")
	idx, err := strconv.Atoi(raw)
	if err != nil || idx < 0 {
		redirectFlash(w, r, "/", "err", "not_found")
		return 0, false
	}
	return idx, true
}
