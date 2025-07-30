package main

import (
	"fyne.io/fyne/v2/app"
)

func main() {
	myApp := app.New()
	w := myApp.NewWindow("Image")
	w.ShowAndRun()
}
