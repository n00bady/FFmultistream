package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func startFFmpeg(appState *AppState) error {
	appState.ctx, appState.cancel = context.WithCancel(context.Background())
	appState.running = true

	args := []string{
		"-fflags", "nobuffer", // minimal buffering reduces input latency
		"-flags", "low_delay", // I don't think this actually helps in this instance since I do not re-encode streams
		"-thread_queue_size", "1024", // max number of packets that demuxer/reader will buffer for an input, usefull for high bitrate streams
		"-protocol_whitelist", "file,http,https,tcp,tls,rtmp,rtmps,crypto",
		"-listen", "1", // listen for the OBS stream
		"-timeout", "10", // listening timeout ffmpeg exits after a few minutes not imediatly apparently
		"-i", appState.config.Origin, // input stream
		"-map", "0:v", // ffmpeg has trouble auto-mapping one input to multiple tee outputs
		"-map", "0:a", // ffmpeg has trouble auto-mapping one input to multiple tee outputs
		"-c:v", "copy", // copy video, NO re-encoding
		"-c:a", "copy", // copy audio, NO re-encoding
		"-f", "tee", // use tee muxer to split output
	}

	var teeOutputs []string
	for i, d := range appState.config.Destinations {
		teeOutputs = append(teeOutputs, fmt.Sprintf("[f=flv:onfail=ignore]%s/%s", d, appState.config.Keys[i]))
	}
	// single string for tee
	teeString := strings.Join(teeOutputs, "|")
	args = append(args, teeString)

	log.Println("ffmpeg", args)

	// execute ffmpeg command with arguments
	cmd := exec.CommandContext(appState.ctx, "ffmpeg", args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		appState.running = false
		appState.cancel = nil
		return fmt.Errorf("failed to start FFmpeg: %v", err)
	}

	log.Printf("Starting FFmpeg with PID: %d", cmd.Process.Pid)
	log.Printf("Pushing streams from %s to %v", appState.config.Origin, appState.config.Destinations)

	if err := cmd.Wait(); err != nil && appState.ctx.Err() != context.Canceled {
		return fmt.Errorf("FFmpeg exited with error: %v", err)
	}

	appState.running = false
	appState.cancel = nil

	return nil
}

func stopFFmpeg(appState *AppState) error {
	if appState.cancel != nil {
		appState.cancel()
		appState.running = false
		appState.cancel = nil
		log.Println("FFmpeg process stoppped.")
	}

	return nil
}
