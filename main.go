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
	"sync"
	"time"

	"github.com/docker/docker/pkg/term"
	"github.com/kr/pty"
	"github.com/ogier/pflag"
)

var (
	mutex       = new(sync.Mutex)
	connMutex   = new(sync.Mutex)
	connections = []net.Conn{}
)

var (
	caCertPath     = pflag.String("ca", "ca.crt", "Path to CA Certificate")
	serverCertPath = pflag.StringP("cert", "c", "server.crt", "Path to server certificate")
	serverKeyPath  = pflag.StringP("key", "k", "server.key", "Path to server key")
)

func main() {
	pflag.Parse()

	if pflag.NArg() != 2 {
		fmt.Printf("usage: %s [ip:port] [program]\n", os.Args[0])
		os.Exit(1)
	}

	s, err := term.MakeRaw(0)

	if err != nil {
		panic(err)
	}

	cmd := exec.Command(pflag.Arg(1))
	pty, err := pty.Start(cmd)

	ws, err := term.GetWinsize(0)
	if err != nil {
		panic(err)
	}

	if err := term.SetWinsize(pty.Fd(), ws); err != nil {
		panic(err)
	}

	go func() {
		cmd.Wait()
		pty.Close()
		term.RestoreTerminal(0, s)

		fmt.Println()
		fmt.Println("Shell exited!")
		os.Exit(0)
	}()

	cert, err := tls.LoadX509KeyPair(*serverCertPath, *serverKeyPath)
	if err != nil {
		fmt.Println("Error", err)
		fmt.Println("You can use generate_cert to generate your certificates: ")
		fmt.Println("`go get github.com/SvenDowideit/generate_cert`")
		os.Exit(1)
	}

	content, err := ioutil.ReadFile(*caCertPath)
	if err != nil {
		panic(err)
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
		panic(err)
	}

	input := new(bytes.Buffer)
	output := new(bytes.Buffer)

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				fmt.Println(err)
				continue
			}

			connMutex.Lock()
			connections = append(connections, c)
			connMutex.Unlock()

			buf := make([]byte, 4)
			if n, err := c.Read(buf); n != 4 || err != nil {
				fmt.Println(err)
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
