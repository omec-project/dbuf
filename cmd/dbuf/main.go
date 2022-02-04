// Copyright 2020-present Open Networking Foundation
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"github.com/omec-project/dbuf"
	"github.com/omec-project/dbuf/utils"
	log "github.com/sirupsen/logrus"
)

var (
	logLevel  = utils.NewLogLevelFlagValue(log.InfoLevel)
	logFormat = utils.NewLogFormatFlagValue(&log.TextFormatter{})
)

func init() {
	flag.Var(logFormat, "log_format", "Format of the logs")
	flag.Var(logLevel, "log_level", "Verbosity of the logs")
}

func main() {
	flag.Parse()
	log.SetReportCaller(true)
	log.SetFormatter(logFormat.GetFormatter())
	log.SetLevel(logLevel.GetLevel())

	dbuffer := dbuf.NewDbuf()
	// Blocking
	err := dbuffer.Run()
	if err != nil {
		log.Fatalf("Error while running Dbuf: %v", err)
	}
}
