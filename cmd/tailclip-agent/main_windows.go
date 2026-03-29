//go:build windows

package main

import (
	"flag"
	"log"

	"tailclip/internal/ui"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	if err := ui.Run(*configPath); err != nil {
		log.Fatal(err)
	}
}
