package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-sigChan:
			log.Printf("Termination signal recieved stopping FFmpeg...")
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				log.Printf("Error stopping FFmpeg: %v", err)
			}
		case <-ctx.Done():
			log.Printf("Context terminated, stopping FFmpeg...")
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				log.Printf("Error stopping FFmpeg: %v", err)
			}
		}
	}()
	wg.Wait()

	return nil
}
