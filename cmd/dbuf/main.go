package main

import (
	"flag"
	"github.com/omec-project/dbuf"
	"log"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	flag.Parse()

	dbuffer := dbuf.NewDbuf()
	// Blocking
	err := dbuffer.Run()
	if err != nil {
		log.Fatalf("Error while running Dbuf: %v", err)
	}
}
