package tperror

import (
	"fmt"
	"os"
)

type TPError struct {
	Message  string
	ExitCode uint8
}

var (
	errorChan chan TPError
)

const (
	ErrUsage    uint8 = 1
	ErrTerminal       = 1 << iota
	ErrCommand        = 1 << iota
	ErrTLS            = 1 << iota
	ErrNetwork        = 1 << iota
)

func ErrorOut(e *TPError) {
	fmt.Fprintf(os.Stderr, e.Message)
	os.Exit(int(e.ExitCode))
}
