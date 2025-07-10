package main

import (
	"fmt"
	"log"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func mainView(appState *AppState) (fyne.CanvasObject, error) {
	log.Println("Creating mainView...")

	dest := appState.config.Destinations

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
			label.SetText(fmt.Sprintf("rtmp URL: %s", d))
		},
	)

	body := container.NewVScroll(list)

	return body, nil
}
