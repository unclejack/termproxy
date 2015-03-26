package main

import (
	"io"
	"net"
	"os"

	"github.com/docker/docker/pkg/term"
)

func main() {
	done := make(chan struct{})

	c, err := net.Dial("tcp", "localhost:4567")
	if err != nil {
		panic(err)
	}

	s, err := term.MakeRaw(0)
	if err != nil {
		panic(err)
	}

	ws, err := term.GetWinsize(0)
	if err != nil {
		panic(err)
	}

	if _, err := c.Write([]byte{byte(ws.Height & 0xFF), byte((ws.Height & 0xFF00) >> 8), byte(ws.Width & 0xFF), byte((ws.Width & 0xFF00) >> 8)}); err != nil {
		panic(err)
	}

	go func() {
		io.Copy(os.Stdout, c)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(c, os.Stdin)
		done <- struct{}{}
	}()

	for i := 0; i < 2; i++ {
		<-done
	}

	term.RestoreTerminal(0, s)
}
