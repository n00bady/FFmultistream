package main

import (
	"fmt"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func mainView(appState *AppState) (fyne.CanvasObject, error) {
	log.Println("Creating mainView...")

	items := binding.NewStringList()
	items.Set(appState.config.Destinations)

	list := widget.NewListWithData(
		items,
		func() fyne.CanvasObject {
			label := widget.NewLabel("Template")
			editBtn := widget.NewButton("Edit", nil)
			delBtn := widget.NewButton("Delete", nil)
			return container.NewHBox(label, layout.NewSpacer(), editBtn, delBtn)
		},
		func(lii binding.DataItem, co fyne.CanvasObject) {
			hbox := co.(*fyne.Container)
			label := hbox.Objects[0].(*widget.Label)
			editBtn := hbox.Objects[2].(*widget.Button)
			delBtn := hbox.Objects[3].(*widget.Button)

			str, _ := lii.(binding.String).Get()
			label.SetText(fmt.Sprintf("%s", str))

			// gets the index of the item
			// because I don't know another way to get it from a binding.DataItem
			listItems, _ := items.Get()
			index := -1
			for i, val := range listItems {
				if val == str {
					index = i
					break
				}
			}

			editBtn.OnTapped = func() {
				if index >= 0 && index < len(listItems) {
					log.Println("Opening edit popup.")
					editPopup(appState, index)
					items.Set(appState.config.Destinations)
				}
			}

			delBtn.OnTapped = func() {
				if index >= 0 && index < len(listItems) {
					appState.config.Destinations = append(appState.config.Destinations[:index], appState.config.Destinations[index+1:]...)
					appState.config.Keys = append(appState.config.Keys[:index], appState.config.Keys[index+1:]...)
					if err := SaveConfig(appState.config); err != nil {
						dialog.ShowError(err, appState.window)
					}
					log.Println("Config saved.")
					items.Set(appState.config.Destinations)
					log.Printf("Deleted destination %d and its key.\n", index)
				}
			}
		},
	)
	listContainer := container.NewVScroll(list)
	listContainer.SetMinSize(fyne.NewSize(350, 500))

	rtmpLabel := widget.NewLabel("RTMP: ")
	rtmpEntry := widget.NewEntry()

	keyLabel := widget.NewLabel("KEY: ")
	keyEntry := widget.NewEntry()

	entriesContainer := container.New(layout.NewFormLayout(), rtmpLabel, rtmpEntry, keyLabel, keyEntry)

	addBtn := widget.NewButton("Add", func() {
		// TODO: Checks for valid inputs ?
		appState.config.Destinations = append(appState.config.Destinations, rtmpEntry.Text)
		appState.config.Keys = append(appState.config.Keys, keyEntry.Text)
		if err := SaveConfig(appState.config); err != nil {
			log.Println("Could not save new config.")
		} else {
			rtmpEntry.SetText("")
			keyEntry.SetText("")
			log.Println("New Destination and Key saved.")
			list.Refresh()
		}
		items.Set(appState.config.Destinations)
	})
	addBtnContainer := container.New(layout.NewHBoxLayout(),
		layout.NewSpacer(),
		addBtn,
	)

	startBtn := widget.NewButton("Start!",
		func() {
			log.Println("Starting pushing origin stream to destinations...")
			go startFFmpeg(appState)
		},
	)
	stopBtn := widget.NewButton("Stop!", func() {
		log.Println("Stopping ffmpeg...")
		stopFFmpeg(appState)
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

func editPopup(appState *AppState, index int) {
	rtmpLabel := widget.NewLabel("RTMP: ")
	rtmpEntry := widget.NewEntry()
	rtmpEntry.Text = appState.config.Destinations[index]
	rtmpEntry.Resize(fyne.NewSize(300, rtmpEntry.MinSize().Height))

	keyLabel := widget.NewLabel("KEY: ")
	keyEntry := widget.NewEntry()
	keyEntry.Text = appState.config.Keys[index]
	keyEntry.Resize(fyne.NewSize(300, keyEntry.MinSize().Height))

	saveBtn := widget.NewButton("Save", func() {
		dialog.ShowInformation("WIP", "NOT IMPLEMENTED YET", appState.window)
	})

	closeBtn := widget.NewButton("Close", nil)	

	entriesContainer := container.New(layout.NewFormLayout(), 
		rtmpLabel, 
		rtmpEntry, 
		keyLabel, 
		keyEntry,
	)

	btnContainer := container.New(layout.NewHBoxLayout(),
		layout.NewSpacer(),
		container.NewPadded(saveBtn),
		layout.NewSpacer(),
		container.NewPadded(closeBtn),
		layout.NewSpacer(),
	)

	content := container.NewVBox(layout.NewSpacer(), entriesContainer, btnContainer) 

	popup := widget.NewModalPopUp(content, appState.window.Canvas())
	popup.Resize(fyne.NewSize(500, 150))

	closeBtn.OnTapped = func() {
		log.Println("Closing edit popup.")
		popup.Hide()
	}

	saveBtn.OnTapped = func() {
		appState.config.Destinations[index] = rtmpEntry.Text
		appState.config.Keys[index] = keyEntry.Text
		SaveConfig(appState.config)
		popup.Hide()
	}

	popup.Show()
}
