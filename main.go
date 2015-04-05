package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/docker/docker/pkg/term"
	"github.com/kr/pty"
	"github.com/ogier/pflag"
)

type tpError struct {
	message  string
	exitcode uint8
}

const (
	ErrUsage    uint8 = 1
	ErrTerminal       = 1 << iota
	ErrCommand        = 1 << iota
	ErrTLS            = 1 << iota
	ErrNetwork        = 1 << iota
)

var (
	mutex       = new(sync.Mutex)
	connMutex   = new(sync.Mutex)
	connections = []net.Conn{}
)

var (
	errorChan   chan tpError
	windowState *term.State
)

var (
	caCertPath     = pflag.String("ca", "ca.crt", "Path to CA Certificate")
	serverCertPath = pflag.StringP("cert", "c", "server.crt", "Path to server certificate")
	serverKeyPath  = pflag.StringP("key", "k", "server.key", "Path to server key")
)

func diag(m error) {
	fmt.Fprintf(os.Stderr, "%v", m)
}

func respondToErrorChan() {
	if errorChan == nil {
		errorChan = make(chan tpError)
	}

	errorOut(<-errorChan)
}

func errorToChan(e tpError) {
	errorChan <- e
}

func errorOut(e tpError) {
	if err := term.RestoreTerminal(0, windowState); err != nil {
		errorOut(tpError{fmt.Sprintf("Could not restore the terminal size during exit: %v", err), ErrTerminal})
	}

	fmt.Fprintf(os.Stderr, e.message)
	os.Exit(int(e.exitcode))
}

func main() {
	pflag.Usage = func() {
		fmt.Printf("usage: %s <options> [host] [program]\n", filepath.Base(os.Args[0]))
		pflag.PrintDefaults()
		os.Exit(int(ErrUsage))
	}

	pflag.Parse()

	if pflag.NArg() != 2 {
		pflag.Usage()
	}

	windowState, err := term.MakeRaw(0)
	if err != nil {
		errorOut(tpError{fmt.Sprintf("Could not create a raw terminal: %v", err), ErrTerminal})
	}

	cmd := exec.Command(pflag.Arg(1))
	pty, err := pty.Start(cmd)
	if err != nil {
		errorOut(tpError{fmt.Sprintf("Could not start program %s: %v", cmd, err), ErrCommand})
	}

	ws, err := term.GetWinsize(0)
	if err != nil {
		errorOut(tpError{fmt.Sprintf("Could not retrieve the terminal dimensions: %v", err), ErrTerminal})
	}

	if err := term.SetWinsize(pty.Fd(), ws); err != nil {
		errorOut(tpError{fmt.Sprintf("Could not set the terminal size of the PTY: %v", err), ErrTerminal})
	}

	go func() {
		cmd.Wait()
		pty.Close()

		if err := term.RestoreTerminal(0, windowState); err != nil {
			errorOut(tpError{fmt.Sprintf("Could not restore the terminal size during exit: %v", err), ErrTerminal})
		}

		fmt.Println()
		fmt.Println("Shell exited!")

		os.Exit(0)
	}()

	cert, err := tls.LoadX509KeyPair(*serverCertPath, *serverKeyPath)
	if err != nil {
		errorOut(tpError{fmt.Sprintf("TLS certificate load error for %s, %s: %v", err), ErrTLS})
	}

	content, err := ioutil.ReadFile(*caCertPath)
	if err != nil {
		errorOut(tpError{fmt.Sprintf("TLS certificate load error for %s: %v", err), ErrTLS})
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(content)

	l, err := tls.Listen("tcp", pflag.Arg(0), &tls.Config{
		RootCAs:      pool,
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})

	if err != nil {
		errorOut(tpError{fmt.Sprintf("Network Error trying to listen on %s: %v", pflag.Arg(0)), ErrNetwork})
	}

	go respondToErrorChan()

	input := new(bytes.Buffer)
	output := new(bytes.Buffer)

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				diag(err)
				continue
			}

			connMutex.Lock()
			connections = append(connections, c)
			connMutex.Unlock()

			buf := make([]byte, 4)
			if n, err := c.Read(buf); n != 4 || err != nil {
				diag(err)
				continue
			}

			ws := term.Winsize{}
			ws.Height = (uint16(buf[1]) << 8) + uint16(buf[0])
			ws.Width = (uint16(buf[3]) << 8) + uint16(buf[2])
			term.SetWinsize(pty.Fd(), &ws)

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
		}
	}()

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
			n, err := pty.Read(buf)
			if err != nil {
				return
			}

			mutex.Lock()
			output.Write(buf[:n])
			mutex.Unlock()
		}
	}()

	// there's gotta be a good way to do this in an evented/blocking manner. This
	// is a big CPU hog right now.
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

			connMutex.Lock()
			for i, c := range connections {
				if _, err := c.Write(output.Bytes()); err != nil {
					if len(connections)+1 > len(connections) {
						connections = connections[:i]
					} else {
						connections = append(connections[:i], connections[i+1:]...)
					}
				}
			}
			connMutex.Unlock()

			if _, err := os.Stdout.Write(output.Bytes()); err != nil {
				break
			}

			output.Reset()
			mutex.Unlock()
		}

		time.Sleep(20 * time.Millisecond)
	}
}
