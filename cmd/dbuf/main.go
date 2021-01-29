// Copyright 2020-present Open Networking Foundation
// SPDX-License-Identifier: LicenseRef-ONF-Member-1.0

package main

import (
	"flag"
	"github.com/omec-project/dbuf"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	flag.Parse()

	dbuffer := dbuf.NewDbuf()
	// Blocking
	err := dbuffer.Run()
	if err != nil {
		log.Fatalf("Error while running Dbuf: %v", err)
	}
}
