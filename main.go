package main

import (
	"context"
	"fmt"
	"log"
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
		"-f", "tee", // use tee muxer to split output
	}

	var teeOutputs []string
	for i, d := range config.Destinations {
		teeOutputs = append(teeOutputs, fmt.Sprintf("[f=flv]%s/%s", d, config.Keys[i]))
	}
	teeString := fmt.Sprintf("%s|%s", teeOutputs[0], teeOutputs[1]) // Joins them for tee muxer
	args = append(args, teeString)

	log.Println("ffmpeg", args)

	// assemble ffmpeg command
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %v", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start ffmpeg: %v", err)
	}

	log.Printf("Starting FFmpeg with PID: %d", cmd.Process.Pid)
	log.Printf("Pushing streams from %s to %v", config.Origin, config.Destinations)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		buff := make([]byte, 1024)
		for {
			n, err := stdout.Read(buff)
			if err != nil {
				return
			}
			if n > 0 {
				log.Printf("FFmpeg stdout: %s", string(buff[:n]))
			}
		}
	}()

	go func() {
		defer wg.Done()

		buff := make([]byte, 1024)
		for {
			n, err := stderr.Read(buff)
			if err != nil {
				return
			}
			if n > 0 {
				log.Printf("FFmpeg stderr: %s", string(buff[:n]))
			}
		}
	}()

	err = cmd.Wait()
	wg.Wait()

	if err != nil {
		return fmt.Errorf("FFmpeg exited with error: %v", err)
	}

	log.Printf("FFmpeg process finished.")

	return nil
}
