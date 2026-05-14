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
		if i < len(cfg.Enabled) && !cfg.Enabled[i] {
			continue
		}
		teeOutputs = append(teeOutputs, fmt.Sprintf("[f=flv:onfail=ignore]%s/%s", d, cfg.Keys[i]))
	}
	args = append(args, strings.Join(teeOutputs, "|"))
	return args
}

func enabledCount(cfg Config) int {
	n := 0
	for i := range cfg.Destinations {
		if i >= len(cfg.Enabled) || cfg.Enabled[i] {
			n++
		}
	}
	return n
}

func startFFmpeg(appState *AppState) error {
	appState.mu.Lock()
	if appState.running {
		appState.mu.Unlock()
		return errAlreadyRunning
	}
	if enabledCount(appState.config) == 0 {
		appState.mu.Unlock()
		return errors.New("no enabled destinations")
	}
	ctx, cancel := context.WithCancel(context.Background())
	appState.ctx = ctx
	appState.cancel = cancel
	appState.running = true
	appState.done = make(chan struct{})
	args := buildFFmpegArgs(appState.config)
	appState.mu.Unlock()

	log.Println("ffmpeg", args)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	markStopped := func() {
		appState.mu.Lock()
		appState.running = false
		appState.cancel = nil
		done := appState.done
		appState.done = nil
		appState.mu.Unlock()
		if done != nil {
			close(done)
		}
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		markStopped()
		return fmt.Errorf("failed to get stderr pipe: %v", err)
	}
	cmd.Stdout = os.Stdout

	if err := cmd.Start(); err != nil {
		markStopped()
		return fmt.Errorf("failed to start FFmpeg: %v", err)
	}

	log.Printf("Starting FFmpeg with PID: %d", cmd.Process.Pid)
	log.Printf("Pushing streams from %s to %v", appState.config.Origin, appState.config.Destinations)

	go scanIntoLogs(stderrPipe, appState.logs)

	waitErr := cmd.Wait()
	ctxErr := ctx.Err()
	markStopped()

	if waitErr != nil && ctxErr != context.Canceled {
		return fmt.Errorf("FFmpeg exited with error: %v", waitErr)
	}
	return nil
}

func stopFFmpeg(appState *AppState) <-chan struct{} {
	appState.mu.Lock()
	cancel := appState.cancel
	done := appState.done
	appState.mu.Unlock()
	if cancel != nil {
		cancel()
		log.Println("FFmpeg process stopped.")
	}
	return done
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
