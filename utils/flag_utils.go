package utils

import (
	"fmt"
	log "github.com/sirupsen/logrus"
)

type LogFormatFlagValue struct {
	formatter log.Formatter
}

type LogLevelFlagValue struct {
	level log.Level
}

func NewLogFormatFlagValue(formatter log.Formatter) *LogFormatFlagValue {
	return &LogFormatFlagValue{formatter: formatter}
}

func NewLogLevelFlagValue(level log.Level) *LogLevelFlagValue {
	return &LogLevelFlagValue{level: level}
}

func (v *LogFormatFlagValue) String() string {
	if _, ok := v.formatter.(*log.TextFormatter); ok {
		return "text"
	}
	if _, ok := v.formatter.(*log.JSONFormatter); ok {
		return "json"
	}

	return "unknown"
}

func (v *LogFormatFlagValue) Set(s string) error {
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

func (v *LogFormatFlagValue) GetFormatter() log.Formatter {
	return v.formatter
}

func (v *LogLevelFlagValue) String() string {
	return v.level.String()
}

func (v *LogLevelFlagValue) Set(s string) error {
	if level, err := log.ParseLevel(s); err != nil {
		return err
	} else {
		v.level = level
	}

	return nil
}

func (v *LogLevelFlagValue) GetLevel() log.Level {
	return v.level
}
