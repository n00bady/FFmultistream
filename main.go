package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	AppInst, err := InitApp(sigChan)
	if err != nil {
		log.Fatalf("GUI initialization failed: %v\n", err)
		AppInst.cancel()
	}

	var body fyne.CanvasObject
	body, err = mainView(AppInst)
	if err != nil {
		log.Fatalf("mainView failed: %v\n", err)
	}

	log.Println("Setting fyne App window content...")
	AppInst.window.SetContent(body)

	AppInst.window.Resize(fyne.NewSize(600, 600))

	log.Println("Running the fyne App...")
	AppInst.window.ShowAndRun()
}

func InitApp(sigChan chan os.Signal) (*AppState, error) {
	myApp := app.NewWithID("FFmultistream")
	myWindow := myApp.NewWindow("FFmultistream")

	appState := &AppState{}

	config, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("Cannot load configuration: %v\n", err)
	}

	appState.running = false
	appState.config = config
	appState.window = myWindow

	myWindow.SetCloseIntercept(func() {
		stopStreaming(appState)
		myWindow.Close()
	})

	go func() {
		<-sigChan
		log.Println("Termination signal received stopping FFmpeg and closing the App...")
		stopStreaming(appState)
		os.Exit(0)
	}()

	return appState, nil
}

func stopStreaming(appState *AppState) error {
	if appState.cancel != nil {
		appState.cancel()
		appState.running = false
		appState.cancel = nil
		log.Println("FFmpeg process stoppped.")
	}

	return nil
}

func startStreaming(appState *AppState) error {
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
