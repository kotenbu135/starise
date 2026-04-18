package main

import (
	"log"

	"github.com/kotenbu135/starise/batch/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
