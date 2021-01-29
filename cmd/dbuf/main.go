// Copyright 2020-present Open Networking Foundation
// SPDX-License-Identifier: LicenseRef-ONF-Member-1.0

package main

import (
	"flag"
	"github.com/omec-project/dbuf"
	log "github.com/sirupsen/logrus"
)

var (
	logLevel  = flag.String("log_level", "debug", "Verbosity of the logs")
	logFormat = flag.String("log_format", "text", "Format of the logs")
)

func main() {
	flag.Parse()
	log.SetReportCaller(true)
	log.SetFormatter(&log.JSONFormatter{})
	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Fatal("Could not parse log level:", err)
	}
	log.SetLevel(level)

	dbuffer := dbuf.NewDbuf()
	// Blocking
	err = dbuffer.Run()
	if err != nil {
		log.Fatalf("Error while running Dbuf: %v", err)
	}
}
