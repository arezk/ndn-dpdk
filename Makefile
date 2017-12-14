PROJNAME=ndn-traffic-dpdk

all: go-dpdk go-ndn

go-dpdk: dpdk/*.go
	go build ./dpdk

build-c/lib$(PROJNAME)-dpdk.a: dpdk/*.c
	./build-c.sh dpdk

go-ndn: ndn/*.go ndn/error.go ndn/tlv-type.go build-c/lib$(PROJNAME)-dpdk.a
	go build ./ndn

ndn/error.go ndn/error.h: ndn/make-error.sh ndn/error.tsv
	ndn/make-error.sh

ndn/tlv-type.go ndn/tlv-type.h: ndn/make-tlv-type.sh ndn/tlv-type.tsv
	ndn/make-tlv-type.sh

test:
	./gotest.sh dpdk
	./gotest.sh ndn
	integ/run.sh

clean:
	rm -rf build-c ndn/error.go ndn/error.h ndn/tlv-type.go ndn/tlv-type.h
	go clean ./...

doxygen:
	cd docs && doxygen Doxyfile 2>&1 | ./filter-Doxygen-warning.awk 1>&2

dochttp: doxygen
	cd docs/html && python3 -m http.server 2>/dev/null &