package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
)

func startFFmpeg(appState *AppState) error {
	appState.ctx, appState.cancel = context.WithCancel(context.Background())
	appState.running = true

	args := []string{
		"-listen", "1", // listen for the OBS stream
		"-timeout", "10", // listening timeout ffmpeg exits after a few minutes not imediatly apparently
		"-i", appState.config.Origin, // input stream
		"-c:v", "copy", // copy video, NO re-encoding
		"-c:a", "copy", // copy audio, NO re-encoding
		"-f", "flv", // use tee muxer to split output
	}

	var teeOutputs []string
	for i, d := range appState.config.Destinations {
		teeOutputs = append(teeOutputs, fmt.Sprintf("[f=flv]%s/%s", d, appState.config.Keys[i]))
	}
	teeString := "tee:"
	for i, t := range teeOutputs {
		if i == len(teeOutputs)-1 {
			teeString = teeString + t
		} else {
			teeString = teeString + t + " | "
		}
	}
	args = append(args, teeString)

	log.Println("ffmpeg", args)

	// assemble ffmpeg command
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
