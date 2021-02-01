// Copyright 2020-present Open Networking Foundation
// SPDX-License-Identifier: LicenseRef-ONF-Member-1.0

package main

import (
	"flag"
	"fmt"
	"github.com/omec-project/dbuf"
	log "github.com/sirupsen/logrus"
)

var (
	logLevel  = flag.String("log_level", "debug", "Verbosity of the logs")
	logFormat = &FormatFlagValue{&log.TextFormatter{}}
)

func init() {
	flag.Var(logFormat, "log_format", "Format of the logs")
}

type FormatFlagValue struct {
	formatter log.Formatter
}

func (v *FormatFlagValue) String() string {
	if _, ok := v.formatter.(*log.TextFormatter); ok {
		return "text"
	}
	if _, ok := v.formatter.(*log.JSONFormatter); ok {
		return "json"
	}

	return "unknown"
}

func (v *FormatFlagValue) Set(s string) error {
	switch s {
	case "text":
		v.formatter = &log.TextFormatter{}
	case "json":
		v.formatter = &log.JSONFormatter{}
	default:
		return fmt.Errorf("allowed values are: text, json")
	}

	return nil
}

func (v *FormatFlagValue) GetFormatter() log.Formatter {
	return v.formatter
}

func main() {
	flag.Parse()
	log.SetReportCaller(true)
	log.SetFormatter(logFormat.GetFormatter())
	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Fatalf("Could not parse log level: %v", err)
	}
	log.SetLevel(level)

	dbuffer := dbuf.NewDbuf()
	// Blocking
	err = dbuffer.Run()
	if err != nil {
		log.Fatalf("Error while running Dbuf: %v", err)
	}
}
