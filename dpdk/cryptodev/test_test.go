package cryptodev_test

import (
	"os"
	"testing"

	"github.com/usnistgov/ndn-dpdk/core/testenv"
	"github.com/usnistgov/ndn-dpdk/dpdk/ealtestenv"
	"github.com/usnistgov/ndn-dpdk/dpdk/pktmbuf/mbuftestenv"
)

func TestMain(m *testing.M) {
	ealtestenv.Init()
	os.Exit(m.Run())
}

var (
	makeAR       = testenv.MakeAR
	bytesFromHex = testenv.BytesFromHex
	makePacket   = mbuftestenv.MakePacket
)
