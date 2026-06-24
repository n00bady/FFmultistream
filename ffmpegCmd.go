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

func buildFFmpegArgs(o Origin) []string {
	args := []string{
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-thread_queue_size", "1024",
		"-protocol_whitelist", "file,http,https,tcp,tls,rtmp,rtmps,crypto",
		"-listen", "1",
		"-timeout", "10",
		"-i", o.URL,
		"-map", "0:v",
		"-map", "0:a",
		"-c:v", "copy",
		"-c:a", "copy",
		"-f", "tee",
	}

	var teeOutputs []string
	for _, d := range o.Destinations {
		if !d.Enabled {
			continue
		}
		teeOutputs = append(teeOutputs, fmt.Sprintf("[f=flv:onfail=ignore]%s/%s", d.RTMP, d.Key))
	}
	args = append(args, strings.Join(teeOutputs, "|"))
	return args
}

func enabledCount(o Origin) int {
	n := 0
	for _, d := range o.Destinations {
		if d.Enabled {
			n++
		}
	}
	return n
}

func startFFmpeg(appState *AppState, oid string) error {
	ctx, args, logs, err := appState.BeginRun(oid)
	if err != nil {
		return err
	}
	defer appState.MarkStopped(oid)

	log.Printf("ffmpeg [origin=%s] %v", oid, args)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %v", err)
	}
	cmd.Stdout = os.Stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %v", err)
	}

	log.Printf("Starting FFmpeg [origin=%s] with PID: %d", oid, cmd.Process.Pid)
	go scanIntoLogs(stderrPipe, logs)

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
