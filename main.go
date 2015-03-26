package main

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/docker/docker/pkg/term"
	"github.com/kr/pty"
)

var mutex sync.Mutex

func main() {
	s, err := term.MakeRaw(0)

	if err != nil {
		panic(err)
	}

	cmd := exec.Command("/bin/zsh")
	pty, err := pty.Start(cmd)

	go func() {
		cmd.Wait()
		pty.Close()
		term.RestoreTerminal(0, s)
		os.Exit(0)
	}()

	l, err := net.Listen("tcp", "localhost:4567")
	if err != nil {
		panic(err)
	}

	c, err := l.Accept()
	if err != nil {
		panic(err)
	}

	buf := make([]byte, 4)
	if n, err := c.Read(buf); n != 4 || err != nil {
		panic(err)
	}

	ws := term.Winsize{}
	ws.Height = (uint16(buf[1]) << 8) + uint16(buf[0])
	ws.Width = (uint16(buf[3]) << 8) + uint16(buf[2])
	term.SetWinsize(pty.Fd(), &ws)

	input := new(bytes.Buffer)
	output := new(bytes.Buffer)

	go func() {
		for {
			buf := make([]byte, 256)
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}

			mutex.Lock()
			input.Write(buf[:n])
			mutex.Unlock()
		}
	}()

	go func() {
		for {
			buf := make([]byte, 256)
			n, err := c.Read(buf)
			if err != nil {
				return
			}

			mutex.Lock()
			input.Write(buf[:n])
			mutex.Unlock()
		}
	}()

	go func() {
		for {
			buf := make([]byte, 256)
			n, err := pty.Read(buf)
			if err != nil {
				return
			}

			mutex.Lock()
			output.Write(buf[:n])
			mutex.Unlock()
		}
	}()

	for {
		if input.Len() > 0 {
			mutex.Lock()
			if _, err := pty.Write(input.Bytes()); err != nil {
				break
			}

			input.Reset()
			mutex.Unlock()
		}

		if output.Len() > 0 {
			mutex.Lock()
			if _, err := c.Write(output.Bytes()); err != nil {
				break
			}

			if _, err := os.Stdout.Write(output.Bytes()); err != nil {
				break
			}

			output.Reset()
			mutex.Unlock()
		}

		time.Sleep(1 * time.Millisecond)
	}
}
