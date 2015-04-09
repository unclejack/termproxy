GOPATH=/tmp
PATH=/tmp/bin:$PATH

host=${1:-localhost}

go get github.com/SvenDowideit/generate_cert
generate_cert --cert ca.crt --key ca.key --overwrite
generate_cert --ca ca.crt --ca-key ca.key \
  --cert server.crt --key server.key \
  --host "$host" --overwrite
generate_cert --ca ca.crt --ca-key ca.key \
  --cert client.crt --key client.key \
  --overwrite

cp -v *.crt client.key termproxy-client
