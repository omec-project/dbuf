package main

import (
	"flag"
	"github.com/omec-project/dbuf"
	"log"
)

func main() {
	flag.Parse()

	dbuffer := dbuf.NewDbuf()
	// Blocking
	err := dbuffer.Run()
	if err != nil {
		log.Fatalf("Error while running Dbuf: %v", err)
	}
}
