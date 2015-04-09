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
	"github.com/erikh/termproxy/tperror"
	"github.com/kr/pty"
	"github.com/ogier/pflag"
)

var (
	mutex       = new(sync.Mutex)
	connMutex   = new(sync.Mutex)
	connections = []net.Conn{}
)

var (
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

func errorOut(e tperror.TPError) {
	if err := term.RestoreTerminal(0, windowState); err != nil {
		tperror.ErrorOut(tperror.TPError{fmt.Sprintf("Could not restore the terminal size during exit: %v", err), tperror.ErrTerminal})
	}

	tperror.ErrorOut(e)
}

func main() {
	pflag.Usage = func() {
		fmt.Printf("usage: %s <options> [host] [program]\n", filepath.Base(os.Args[0]))
		pflag.PrintDefaults()
		os.Exit(int(tperror.ErrUsage))
	}

	pflag.Parse()

	if pflag.NArg() != 2 {
		pflag.Usage()
	}

	windowState, err := term.MakeRaw(0)
	if err != nil {
		errorOut(tperror.TPError{fmt.Sprintf("Could not create a raw terminal: %v", err), tperror.ErrTerminal})
	}

	cmd := exec.Command(pflag.Arg(1))
	pty, err := pty.Start(cmd)
	if err != nil {
		errorOut(tperror.TPError{fmt.Sprintf("Could not start program %s: %v", cmd, err), tperror.ErrCommand})
	}

	ws, err := term.GetWinsize(0)
	if err != nil {
		errorOut(tperror.TPError{fmt.Sprintf("Could not retrieve the terminal dimensions: %v", err), tperror.ErrTerminal})
	}

	if err := term.SetWinsize(pty.Fd(), ws); err != nil {
		errorOut(tperror.TPError{fmt.Sprintf("Could not set the terminal size of the PTY: %v", err), tperror.ErrTerminal})
	}

	go func() {
		cmd.Wait()
		// FIXME sloppy as heck but works for now.
		for _, c := range connections {
			c.Close()
		}

		pty.Close()

		if err := term.RestoreTerminal(0, windowState); err != nil {
			errorOut(tperror.TPError{fmt.Sprintf("Could not restore the terminal size during exit: %v", err), tperror.ErrTerminal})
		}

		fmt.Println()
		fmt.Println("Shell exited!")

		os.Exit(0)
	}()

	cert, err := tls.LoadX509KeyPair(*serverCertPath, *serverKeyPath)
	if err != nil {
		errorOut(tperror.TPError{fmt.Sprintf("TLS certificate load error for %s, %s: %v", err), tperror.ErrTLS})
	}

	content, err := ioutil.ReadFile(*caCertPath)
	if err != nil {
		errorOut(tperror.TPError{fmt.Sprintf("TLS certificate load error for %s: %v", err), tperror.ErrTLS})
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
		errorOut(tperror.TPError{fmt.Sprintf("Network Error trying to listen on %s: %v", pflag.Arg(0)), tperror.ErrNetwork})
	}

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
