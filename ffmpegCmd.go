package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

var errAlreadyRunning = errors.New("ffmpeg is already running")

func buildFFmpegArgs(cfg Config) []string {
	args := []string{
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-thread_queue_size", "1024",
		"-protocol_whitelist", "file,http,https,tcp,tls,rtmp,rtmps,crypto",
		"-listen", "1",
		"-timeout", "10",
		"-i", cfg.Origin,
		"-map", "0:v",
		"-map", "0:a",
		"-c:v", "copy",
		"-c:a", "copy",
		"-f", "tee",
	}

	var teeOutputs []string
	for i, d := range cfg.Destinations {
		teeOutputs = append(teeOutputs, fmt.Sprintf("[f=flv:onfail=ignore]%s/%s", d, cfg.Keys[i]))
	}
	args = append(args, strings.Join(teeOutputs, "|"))
	return args
}

func startFFmpeg(appState *AppState) error {
	appState.mu.Lock()
	if appState.running {
		appState.mu.Unlock()
		return errAlreadyRunning
	}
	if len(appState.config.Destinations) == 0 {
		appState.mu.Unlock()
		return errors.New("no destinations configured")
	}
	ctx, cancel := context.WithCancel(context.Background())
	appState.ctx = ctx
	appState.cancel = cancel
	appState.running = true
	args := buildFFmpegArgs(appState.config)
	appState.mu.Unlock()

	log.Println("ffmpeg", args)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		appState.mu.Lock()
		appState.running = false
		appState.cancel = nil
		appState.mu.Unlock()
		return fmt.Errorf("failed to get stderr pipe: %v", err)
	}
	cmd.Stdout = os.Stdout

	if err := cmd.Start(); err != nil {
		appState.mu.Lock()
		appState.running = false
		appState.cancel = nil
		appState.mu.Unlock()
		return fmt.Errorf("failed to start FFmpeg: %v", err)
	}

	log.Printf("Starting FFmpeg with PID: %d", cmd.Process.Pid)
	log.Printf("Pushing streams from %s to %v", appState.config.Origin, appState.config.Destinations)

	go scanIntoLogs(stderrPipe, appState.logs)

	waitErr := cmd.Wait()

	appState.mu.Lock()
	appState.running = false
	appState.cancel = nil
	ctxErr := ctx.Err()
	appState.mu.Unlock()

	if waitErr != nil && ctxErr != context.Canceled {
		return fmt.Errorf("FFmpeg exited with error: %v", waitErr)
	}
	return nil
}

func stopFFmpeg(appState *AppState) error {
	appState.mu.Lock()
	cancel := appState.cancel
	appState.mu.Unlock()
	if cancel != nil {
		cancel()
		log.Println("FFmpeg process stopped.")
	}
	return nil
}

func scanIntoLogs(r io.Reader, buf *ringBuffer) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if buf != nil {
			buf.Append(line)
		}
		fmt.Fprintln(os.Stderr, line)
	}
}
