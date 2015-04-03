# termproxy: share a program with others (for pairing!)

termproxy is a shared program tool. It allows you to start the program of your
choice (a shell, vim/emacs, etc) and allows others to connect and interact with
it. The intended use case is pairing.

## SSL Notice

*NOTE*: termproxy is currently TLS encrypted, but with no authentication. You
must generate a CA and server keypair before you can use this program. It is
not safe to leave this program running against the open internet! This is going
to be fixed with client authentication in a future release.

The easy way:

```shell
$ PATH=$HOME:$PATH
$ GOPATH=$HOME
$ go get github.com/SvenDowideit/generate_cert
$ generate_cert --cert ca.crt  --key ca.key -overwrite
$ generate_cert --ca ca.crt --ca-key ca.key \
  --cert server.crt --key server.key \
  --host localhost --overwrite
```

Then ship the `ca.crt` file to your client users. Note that the files must be
in the current working directory.

## Installation

```shell
# for the server
$ go get github.com/erikh/termproxy
# for the client
$ go get github.com/erikh/termproxy/termproxy-client
```

## Author

Erik Hollensbe <erik@hollensbe.org>
