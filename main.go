package main

import (
	"fmt"
	"log"
	"os"
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

	AppInst.window.Resize(fyne.NewSize(750, 400))

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

	// stopping ffmpeg first then closing the app
	myWindow.SetCloseIntercept(func() {
		stopFFmpeg(appState)
		myWindow.Close()
	})

	// goroutine to catch signals
	go func() {
		<-sigChan
		log.Println("Termination signal received stopping FFmpeg and closing the App...")
		stopFFmpeg(appState)
		os.Exit(0)
	}()

	return appState, nil
}

