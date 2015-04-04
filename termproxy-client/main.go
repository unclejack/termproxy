package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/docker/docker/pkg/term"
	"github.com/ogier/pflag"
)

var (
	caCertPath     = pflag.String("ca", "ca.crt", "Path to CA Certificate")
	serverCertPath = pflag.StringP("servercert", "s", "server.crt", "Path to Server Certificate")
	clientCertPath = pflag.StringP("cert", "c", "client.crt", "Path to Client Certificate")
	clientKeyPath  = pflag.StringP("key", "k", "client.key", "Path to Client Key")
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
	pflag.Parse()

	if pflag.NArg() != 1 {
		fmt.Printf("usage: %s [host]\n", os.Args[0])
		os.Exit(1)
	}

	content, err := ioutil.ReadFile(*caCertPath)
	if err != nil {
		panic(err)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(content)

	content, err = ioutil.ReadFile(*serverCertPath)
	if err != nil {
		panic(err)
	}

	pool.AppendCertsFromPEM(content)

	cert, err := tls.LoadX509KeyPair(*clientCertPath, *clientKeyPath)
	if err != nil {
		panic(err)
	}

	c, err := tls.Dial("tcp", pflag.Arg(0), &tls.Config{
		ClientCAs:    pool,
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
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
