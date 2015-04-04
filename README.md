# termproxy: share a program with others (for pairing!)

**termproxy is currently alpha quality**

termproxy is a shared program tool. It allows you to start the program of your
choice (a shell, vim/emacs, etc) and allows others to connect and interact with
it. The intended use case is pairing.

## SSL Notice

This program makes heavy use of SSL and certificates. Follow the instructions
below to generate a CA, server and client certificate for use.

```bash
$ host=$(cat /etc/hostname)
$ PATH=$HOME:$PATH
$ GOPATH=$HOME
$ go get github.com/SvenDowideit/generate_cert
$ generate_cert --cert ca.crt  --key ca.key -overwrite
$ generate_cert --ca ca.crt --ca-key ca.key \
  --cert server.crt --key server.key \
  --host "$host" --overwrite
$ generate_cert --ca ca.crt --ca-key ca.key \
  --cert client.crt --key client.key \
  --overwrite
```

Then ship the `ca.crt` and `client.*` files to your client users. Note that the
files must be in the current working directory for both the server and the
client.

## Installation

```bash
# for the server
$ go get github.com/erikh/termproxy
# for the client
$ go get github.com/erikh/termproxy/termproxy-client
```

## Author

Erik Hollensbe <erik@hollensbe.org>
