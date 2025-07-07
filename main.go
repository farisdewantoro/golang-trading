package main

import (
	"golang-trading/cmd"
	"log"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatalf("could not start application: %v", err)
	}
}
