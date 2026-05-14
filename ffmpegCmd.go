package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

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
	ctx, args, _, err := appState.BeginRun()
	if err != nil {
		return err
	}
	defer appState.MarkStopped()

	log.Println("ffmpeg", args)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %v", err)
	}
	cmd.Stdout = os.Stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %v", err)
	}

	log.Printf("Starting FFmpeg with PID: %d", cmd.Process.Pid)
	go scanIntoLogs(stderrPipe, appState.logs)

	waitErr := cmd.Wait()
	if waitErr != nil && ctx.Err() != context.Canceled {
		return fmt.Errorf("FFmpeg exited with error: %v", waitErr)
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
