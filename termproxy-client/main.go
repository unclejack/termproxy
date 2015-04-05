package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/docker/docker/pkg/term"
	"github.com/erikh/termproxy/tperror"
	"github.com/ogier/pflag"
)

var (
	windowState *term.State
)

var (
	caCertPath     = pflag.String("ca", "ca.crt", "Path to CA Certificate")
	serverCertPath = pflag.StringP("servercert", "s", "server.crt", "Path to Server Certificate")
	clientCertPath = pflag.StringP("cert", "c", "client.crt", "Path to Client Certificate")
	clientKeyPath  = pflag.StringP("key", "k", "client.key", "Path to Client Key")
)

func errorOut(err tperror.TPError) {
	term.RestoreTerminal(0, windowState)
	tperror.ErrorOut(err)
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

	windowState, err = term.MakeRaw(0)
	if err != nil {
		panic(err)
	}

	ws, err := term.GetWinsize(0)
	if err != nil {
		errorOut(tperror.TPError{fmt.Sprintf("Error getting terminal size: %v", err), tperror.ErrTerminal})
	}

	if _, err := c.Write([]byte{
		byte(ws.Height & 0xFF),
		byte((ws.Height & 0xFF00) >> 8),
		byte(ws.Width & 0xFF),
		byte((ws.Width & 0xFF00) >> 8),
	}); err != nil {
		errorOut(tperror.TPError{fmt.Sprintf("Error writing terminal size to server: %v", err), tperror.ErrNetwork | tperror.ErrTerminal})
	}

	go func() {
		if _, err := io.Copy(os.Stdout, c); err != nil && err != io.EOF {
			errorOut(tperror.TPError{fmt.Sprintf("Error reading from server: %v", err), tperror.ErrNetwork})
		}
	}()

	go func() {
		if _, err := io.Copy(c, os.Stdin); err != nil && err != io.EOF {
			errorOut(tperror.TPError{fmt.Sprintf("Error writing to server: %v", err), tperror.ErrNetwork})
		}
	}()

	select {}

	fmt.Println("Shell exited!")
}
