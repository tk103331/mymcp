package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/tk103331/mymcp/internal/app"
)

func main() {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	if err := app.New().Run(); err != nil {
		log.Fatal(err)
	}
}
