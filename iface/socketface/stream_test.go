package socketface_test

import (
	"net"
	"testing"

	"ndn-dpdk/iface/ifacetestfixture"
	"ndn-dpdk/iface/socketface"
)

func TestStream(t *testing.T) {
	connA, connB := net.Pipe()

	faceA := socketface.New(connA, socketface.Config{
		Mempools:    faceMempools,
		RxMp:        directMp,
		RxqCapacity: 64,
		TxqCapacity: 64,
	})
	faceB := socketface.New(connB, socketface.Config{
		Mempools:    faceMempools,
		RxMp:        directMp,
		RxqCapacity: 64,
		TxqCapacity: 64,
	})
	defer faceA.Close()
	defer faceB.Close()

	fixture := ifacetestfixture.New(t, faceA, socketface.NewRxGroup(faceA), faceB)
	fixture.RunTest()
	fixture.CheckCounters()
}
