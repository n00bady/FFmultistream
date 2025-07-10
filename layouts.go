package main

import (
	"fmt"
	"log"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func mainView(appState *AppState) (fyne.CanvasObject, error) {
	log.Println("Creating mainView...")

	dest := appState.config.Destinations

	// List of destinations
	list := widget.NewList(
		func() int {
			return len(dest)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(lii widget.ListItemID, co fyne.CanvasObject) {
			if len(dest) <= 0 {
				dialog.ShowError(fmt.Errorf("Destinations are empty!"), appState.window)
				os.Exit(1)
			}
			log.Printf("Updating item with ID: %d", lii)
			if lii < 0 || lii >= len(dest) {
				log.Printf("Invalid item ID: %d", lii)
				return
			}
			d := dest[lii]
			label, ok := co.(*widget.Label)
			if !ok {
				log.Printf("Canvas object is not *widget.Label, its: %s", fmt.Sprintf("%T", co))
				return
			}
			label.SetText(fmt.Sprintf("Destination %d: %s", lii, d))
		},
	)
	listContainer := container.NewVScroll(list)
	listContainer.SetMinSize(fyne.NewSize(350, 500))

	// Adding new destinations
	rtmpLabel := widget.NewLabel("rtmp: ")
	rtmpEntry := widget.NewEntry()

	keyLabel := widget.NewLabel("key: ")
	keyEntry := widget.NewEntry()

	entriesContainer := container.New(layout.NewFormLayout(), rtmpLabel, rtmpEntry, keyLabel, keyEntry)

	addBtn := widget.NewButton("Add", func() {
		log.Println("NOT IMPLEMENTED YET!")
	})
	addBtnContainer := container.New(layout.NewHBoxLayout(),
		layout.NewSpacer(),
		addBtn,
	)

	// newEntryContainer := container.New(layout.NewGridLayoutWithRows(3), entriesContainer, layout.NewSpacer(), addBtn)

	// FFmpeg Start and Stop buttons
	startBtn := widget.NewButton("Start!",
		func() {
			log.Println("Starting pushing origin stream to destinations...")
			go startStreaming(appState)
		},
	)
	stopBtn := widget.NewButton("Stop!", func() {
		log.Println("Stopping ffmpeg...")
		stopStreaming(appState)
	})
	btnContainer := container.New(layout.NewHBoxLayout(),
		layout.NewSpacer(),
		container.NewPadded(startBtn),
		layout.NewSpacer(),
		container.NewPadded(stopBtn),
		layout.NewSpacer(),
	)

	body := container.New(layout.NewGridLayoutWithColumns(2), 
		listContainer, 
		container.NewVBox(entriesContainer, 
			addBtnContainer, 
			layout.NewSpacer(), 
			btnContainer, 
			layout.NewSpacer(),
		),
	)

	return body, nil
}
