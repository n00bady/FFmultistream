package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
)

type Config struct {
	Origin       string   `toml:"Origin"`
	Destinations []string `toml:"Destinations"`
	Keys         []string `toml:"Keys"`
}

func main() {
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Cannot load Config!: %v", err)
	}

	if err := startStreaming(context.Background(), config); err != nil {
		log.Printf("Cannot start the streaming process: %v", err)
	}
}

func startStreaming(ctx context.Context, config Config) error {
	args := []string{
		"-listen", "1", // listen for the OBS stream
		"-i", config.Origin, // input stream
		"-c:v", "copy", // copy video, NO re-encoding
		"-c:a", "copy", // copy audio, NO re-encoding
		"-f", "flv", // use tee muxer to split output
	}

	var teeOutputs []string
	for i, d := range config.Destinations {
		teeOutputs = append(teeOutputs, fmt.Sprintf("[f=flv]%s/%s", d, config.Keys[i]))
	}
	teeString := fmt.Sprintf("tee:%s | %s", teeOutputs[0], teeOutputs[1]) // Joins them for tee muxer
	args = append(args, teeString)

	log.Println("ffmpeg", args)

	// assemble ffmpeg command
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %v", err)
	}

	log.Printf("Starting FFmpeg with PID: %d", cmd.Process.Pid)
	log.Printf("Pushing streams from %s to %v", config.Origin, config.Destinations)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := cmd.Wait(); err != nil {
			log.Printf("FFmpeg process exited with error: %v", err)
		} else {
			log.Printf("FFmpeg process finished successfully")
		}
	}()
	wg.Wait()

	return nil
}
