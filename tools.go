package main

import (
	"fmt"
	"strings"
)

// fieldFlag reusable parse Value to create import command
type fieldFlag struct {
	Fields []string
}

func (ff *fieldFlag) String() string {
	return fmt.Sprint(ff.Fields)
}

func (ff *fieldFlag) Set(value string) error {
	if len(ff.Fields) > 0 {
		return fmt.Errorf("The field flag is already set")
	}

	ff.Fields = strings.Split(value, ",")
	return nil
}
