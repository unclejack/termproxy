package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"

	"github.com/docker/docker/pkg/term"
	"github.com/erikh/termproxy/tperror"
	"github.com/ogier/pflag"
	"golang.org/x/sys/unix"
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

func errorOut(err *tperror.TPError) {
	term.RestoreTerminal(0, windowState)
	tperror.ErrorOut(err)
}

func readCerts() (tls.Certificate, *x509.CertPool, *tperror.TPError) {
	content, err := ioutil.ReadFile(*caCertPath)
	if err != nil {
		return tls.Certificate{}, nil, &tperror.TPError{fmt.Sprintf("Could not read CA certificate '%s': %v", *caCertPath, err), tperror.ErrTLS}
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(content)

	content, err = ioutil.ReadFile(*serverCertPath)
	if err != nil {
		return tls.Certificate{}, nil, &tperror.TPError{fmt.Sprintf("Could not read server certificate '%s': %v", *serverCertPath, err), tperror.ErrTLS}
	}

	pool.AppendCertsFromPEM(content)

	cert, err := tls.LoadX509KeyPair(*clientCertPath, *clientKeyPath)
	if err != nil {
		return tls.Certificate{}, nil, &tperror.TPError{fmt.Sprintf("Could not read client keypair '%s' and '%s': %v", *clientCertPath, *clientKeyPath, err), tperror.ErrTLS}
	}

	return cert, pool, nil
}

func connect() net.Conn {
	cert, pool, tperr := readCerts()
	if tperr != nil {
		errorOut(tperr)
	}

	c, err := tls.Dial("tcp", pflag.Arg(0), &tls.Config{
		ClientCAs:    pool,
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		errorOut(&tperror.TPError{fmt.Sprintf("Could not connect to server at %s: %v", pflag.Arg(0), err), tperror.ErrTLS | tperror.ErrNetwork})
	}

	return c
}

func configureTerminal() (*term.Winsize, *tperror.TPError) {
	var err error

	windowState, err = term.MakeRaw(0)
	if err != nil {
		return nil, &tperror.TPError{fmt.Sprintf("Could not create a raw terminal: %v", err), tperror.ErrTerminal}
	}

	ws, err := term.GetWinsize(0)
	if err != nil {
		return nil, &tperror.TPError{fmt.Sprintf("Error getting terminal size: %v", err), tperror.ErrTerminal}
	}

	return ws, nil
}

func writeTermSize(c net.Conn) *tperror.TPError {
	ws, tperr := configureTerminal()
	if tperr != nil {
		return tperr
	}

	if _, err := c.Write([]byte{
		byte(ws.Height & 0xFF),
		byte((ws.Height & 0xFF00) >> 8),
		byte(ws.Width & 0xFF),
		byte((ws.Width & 0xFF00) >> 8),
	}); err != nil {
		return &tperror.TPError{fmt.Sprintf("Error writing terminal size to server: %v", err), tperror.ErrNetwork | tperror.ErrTerminal}
	}

	return nil
}

func copyToStdout(c net.Conn) {
	if _, err := io.Copy(os.Stdout, c); err != nil && err != io.EOF {
		errorOut(&tperror.TPError{fmt.Sprintf("Error reading from server: %v", err), tperror.ErrNetwork})
	}
}

func copyStdin(c net.Conn) {
	if _, err := io.Copy(c, os.Stdin); err != nil && err != io.EOF {
		if neterr, ok := err.(*net.OpError); ok && neterr.Err == unix.EPIPE {
			term.RestoreTerminal(0, windowState)
			fmt.Println("\n\nConnection terminated!")
			os.Exit(0)
		} else {
			errorOut(&tperror.TPError{fmt.Sprintf("Error writing to server: %v", err), tperror.ErrNetwork})
		}
	}
}

func main() {
	pflag.Parse()

	if pflag.NArg() != 1 {
		fmt.Printf("usage: %s [host]\n", os.Args[0])
		pflag.PrintDefaults()
		os.Exit(1)
	}

	c := connect()
	writeTermSize(c)

	go copyToStdout(c)
	go copyStdin(c)

	select {}
}
