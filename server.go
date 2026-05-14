package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookie = "ffms_session"
	sessionTTL    = 7 * 24 * time.Hour
)

type Server struct {
	state    *AppState
	tpl      *templates
	sessions sync.Map
}

func newServer(state *AppState, tpl *templates) *Server {
	return &Server{state: state, tpl: tpl}
}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /destinations", s.handleAddDestination)
	mux.HandleFunc("GET /destinations/new", s.handleAddDestinationForm)
	mux.HandleFunc("GET /destinations/{i}/edit", s.handleEditDestination)
	mux.HandleFunc("POST /destinations/{i}", s.handleUpdateDestination)
	mux.HandleFunc("POST /destinations/{i}/delete", s.handleDeleteDestination)
	mux.HandleFunc("POST /destinations/{i}/toggle", s.handleToggleDestination)
	mux.HandleFunc("GET /origin/edit", s.handleEditOriginForm)
	mux.HandleFunc("POST /origin", s.handleUpdateOrigin)
	mux.HandleFunc("POST /stream/start", s.handleStartStream)
	mux.HandleFunc("POST /stream/stop", s.handleStopStream)
	mux.HandleFunc("GET /logs", s.handleLogs)
	mux.HandleFunc("GET /login", s.handleLoginGET)
	mux.HandleFunc("POST /login", s.handleLoginPOST)
	mux.HandleFunc("POST /logout", s.handleLogout)
	return mux
}

func (s *Server) isAuthenticated(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return false
	}
	_, ok := s.sessions.Load(c.Value)
	return ok
}

func (s *Server) createSession() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("session token: %v", err)
	}
	token := hex.EncodeToString(b)
	s.sessions.Store(token, struct{}{})
	return token
}

func (s *Server) revokeSession(token string) {
	s.sessions.Delete(token)
}

var flashMessages = map[string]string{
	"added":      "Destination added.",
	"saved":      "Saved.",
	"deleted":    "Destination deleted.",
	"started":    "Stream started.",
	"stopped":    "Stream stopped.",
	"paused":     "Destination paused.",
	"resumed":    "Destination resumed.",
	"logged_out": "Signed out.",
}

var flashErrors = map[string]string{
	"invalid_rtmp":    "RTMP URL is not valid.",
	"invalid_key":     "Stream key cannot be empty.",
	"not_found":       "Destination not found.",
	"already_running": "Stream is already running.",
	"not_running":     "Stream is not running.",
	"no_destinations": "No destinations configured.",
	"no_enabled":      "All destinations are paused.",
	"save_failed":     "Failed to save configuration.",
	"start_failed":    "Failed to start ffmpeg.",
	"forbidden":       "Request rejected (cross-origin).",
	"invalid_login":   "Invalid username or password.",
}

type baseView struct {
	Config    Config
	Running   bool
	Flash     string
	FlashErr  string
	NavActive string
}

type indexView struct {
	baseView
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

type loginView struct {
	Flash     string
	FlashErr  string
	NavActive string
}

func (s *Server) snapshot(r *http.Request) baseView {
	cfg, running := s.state.Snapshot()
	return baseView{
		Config:   cfg,
		Running:  running,
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
	base.NavActive = "home"
	s.render(w, s.tpl.index, indexView{baseView: base})
}

func (s *Server) handleAddDestinationForm(w http.ResponseWriter, r *http.Request) {
	base := s.snapshot(r)
	base.NavActive = "home"
	s.render(w, s.tpl.add, base)
}

func (s *Server) handleAddDestination(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		redirectFlash(w, r, "/destinations/new", "err", "invalid_rtmp")
		return
	}
	rtmp := strings.TrimSpace(r.PostFormValue("rtmp"))
	key := strings.TrimSpace(r.PostFormValue("key"))
	if !IsvalidRTMP(rtmp) {
		redirectFlash(w, r, "/destinations/new", "err", "invalid_rtmp")
		return
	}
	if !IsvalidKEY(key) {
		redirectFlash(w, r, "/destinations/new", "err", "invalid_key")
		return
	}

	cfg := s.state.AddDestination(rtmp, key)
	if err := SaveConfig(cfg); err != nil {
		log.Printf("SaveConfig failed: %v", err)
		redirectFlash(w, r, "/destinations/new", "err", "save_failed")
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
	base.NavActive = "home"
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

	cfg, ok := s.state.UpdateDestination(idx, rtmp, key)
	if !ok {
		redirectFlash(w, r, "/", "err", "not_found")
		return
	}
	if err := SaveConfig(cfg); err != nil {
		log.Printf("SaveConfig failed: %v", err)
		redirectFlash(w, r, "/", "err", "save_failed")
		return
	}
	redirectFlash(w, r, "/", "msg", "saved")
}

func (s *Server) handleDeleteDestination(w http.ResponseWriter, r *http.Request) {
	idx, ok := s.parseIndex(w, r)
	if !ok {
		return
	}

	cfg, ok := s.state.DeleteDestination(idx)
	if !ok {
		redirectFlash(w, r, "/", "err", "not_found")
		return
	}
	if err := SaveConfig(cfg); err != nil {
		log.Printf("SaveConfig failed: %v", err)
		redirectFlash(w, r, "/", "err", "save_failed")
		return
	}
	redirectFlash(w, r, "/", "msg", "deleted")
}

func (s *Server) handleEditOriginForm(w http.ResponseWriter, r *http.Request) {
	base := s.snapshot(r)
	base.NavActive = "home"
	s.render(w, s.tpl.origin, base)
}

func (s *Server) handleUpdateOrigin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		redirectFlash(w, r, "/origin/edit", "err", "invalid_rtmp")
		return
	}
	origin := strings.TrimSpace(r.PostFormValue("origin"))
	if !IsvalidRTMP(origin) {
		redirectFlash(w, r, "/origin/edit", "err", "invalid_rtmp")
		return
	}

	cfg := s.state.SetOrigin(origin)
	if err := SaveConfig(cfg); err != nil {
		log.Printf("SaveConfig failed: %v", err)
		redirectFlash(w, r, "/origin/edit", "err", "save_failed")
		return
	}
	redirectFlash(w, r, "/", "msg", "saved")
}

func (s *Server) handleStartStream(w http.ResponseWriter, r *http.Request) {
	if err := s.state.CanStart(); err != nil {
		redirectFlash(w, r, "/", "err", startErrorCode(err))
		return
	}
	go func() {
		if err := startFFmpeg(s.state); err != nil {
			log.Printf("ffmpeg error: %v", err)
		}
	}()
	redirectFlash(w, r, "/", "msg", "started")
}

func (s *Server) handleStopStream(w http.ResponseWriter, r *http.Request) {
	if !s.state.Running() {
		redirectFlash(w, r, "/", "err", "not_running")
		return
	}
	s.state.Stop()
	redirectFlash(w, r, "/", "msg", "stopped")
}

func (s *Server) handleToggleDestination(w http.ResponseWriter, r *http.Request) {
	idx, ok := s.parseIndex(w, r)
	if !ok {
		return
	}

	cfg, nowEnabled, wasRunning, ok := s.state.ToggleDestination(idx)
	if !ok {
		redirectFlash(w, r, "/", "err", "not_found")
		return
	}
	if err := SaveConfig(cfg); err != nil {
		log.Printf("SaveConfig failed: %v", err)
		redirectFlash(w, r, "/", "err", "save_failed")
		return
	}

	if wasRunning {
		if done := s.state.Stop(); done != nil {
			<-done
		}
		if enabledCount(cfg) > 0 {
			go func() {
				if err := startFFmpeg(s.state); err != nil {
					log.Printf("ffmpeg restart error: %v", err)
				}
			}()
		}
	}

	code := "paused"
	if nowEnabled {
		code = "resumed"
	}
	redirectFlash(w, r, "/", "msg", code)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	base := s.snapshot(r)
	base.NavActive = "logs"
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

func (s *Server) handleLoginGET(w http.ResponseWriter, r *http.Request) {
	if s.isAuthenticated(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, s.tpl.login, loginView{
		Flash:     flashMessages[r.URL.Query().Get("msg")],
		FlashErr:  flashErrors[r.URL.Query().Get("err")],
		NavActive: "login",
	})
}

func (s *Server) handleLoginPOST(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		redirectFlash(w, r, "/login", "err", "invalid_login")
		return
	}
	user := r.PostFormValue("username")
	pass := r.PostFormValue("password")
	expU, expP := s.state.Credentials()
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(expU)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(expP)) == 1
	if !userOK || !passOK {
		redirectFlash(w, r, "/login", "err", "invalid_login")
		return
	}
	token := s.createSession()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.revokeSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookie,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	redirectFlash(w, r, "/login", "msg", "logged_out")
}

func startErrorCode(err error) string {
	switch {
	case errors.Is(err, errAlreadyRunning):
		return "already_running"
	case errors.Is(err, errNoDestinations):
		return "no_destinations"
	case errors.Is(err, errNoEnabled):
		return "no_enabled"
	default:
		return "start_failed"
	}
}
