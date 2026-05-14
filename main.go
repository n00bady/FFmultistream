package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", "0.0.0.0:8765", "address to bind the web UI on")
	openBrowser := flag.Bool("open", true, "open the UI in the default browser on start")
	flag.Parse()

	state, err := initApp()
	if err != nil {
		log.Fatalf("initialization failed: %v", err)
	}

	tpl, err := loadTemplates()
	if err != nil {
		log.Fatalf("template loading failed: %v", err)
	}

	srv := newServer(state, tpl)
	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           srv.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Termination signal received, stopping FFmpeg and shutting down...")
		stopFFmpeg(state)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
	}()

	log.Printf("FFmultistream UI is listening on http://%s", *addr)
	if *openBrowser {
		go tryOpenBrowser("http://" + browserHost(*addr))
	}

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("http server error: %v", err)
	}

	stopFFmpeg(state)
	log.Println("Goodbye.")
}

func initApp() (*AppState, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return &AppState{
		config: config,
		logs:   newRingBuffer(500),
	}, nil
}

func browserHost(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		return net.JoinHostPort("127.0.0.1", port)
	}
	return addr
}

func tryOpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	if err := cmd.Start(); err != nil {
		log.Printf("could not open browser automatically: %v (visit %s)", err, url)
	}
}
