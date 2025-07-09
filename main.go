package main

import (
	"log"

	"github-okr-fetcher/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}