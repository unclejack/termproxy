package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/docker/docker/pkg/term"
)

func terminate(s *term.State, err error) {
	term.RestoreTerminal(0, s)
	fmt.Println()
	fmt.Println("Shell exited!")
	if err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("usage: %s [host]\n", os.Args[0])
		os.Exit(1)
	}

	content, err := ioutil.ReadFile("ca.crt")
	if err != nil {
		panic(err)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(content)

	c, err := tls.Dial("tcp", os.Args[1], &tls.Config{RootCAs: pool})
	if err != nil {
		panic(err)
	}

	s, err := term.MakeRaw(0)
	if err != nil {
		panic(err)
	}

	ws, err := term.GetWinsize(0)
	if err != nil {
		terminate(s, err)
	}

	if _, err := c.Write([]byte{
		byte(ws.Height & 0xFF),
		byte((ws.Height & 0xFF00) >> 8),
		byte(ws.Width & 0xFF),
		byte((ws.Width & 0xFF00) >> 8),
	}); err != nil {
		terminate(s, err)
	}

	go func() {
		io.Copy(os.Stdout, c)
		terminate(s, err)
	}()

	go func() {
		io.Copy(c, os.Stdin)
		terminate(s, err)
	}()

	select {}

	fmt.Println("Shell exited!")
}
