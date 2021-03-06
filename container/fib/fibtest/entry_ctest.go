package fibtest

/*
#include "../../../csrc/fib/entry.h"
*/
import "C"
import (
	"testing"
	"unsafe"

	"github.com/usnistgov/ndn-dpdk/container/fib"
	"github.com/usnistgov/ndn-dpdk/iface"
	"github.com/usnistgov/ndn-dpdk/ndn"
	"github.com/usnistgov/ndn-dpdk/ndn/ndntestenv"
)

func ctestEntry(t *testing.T) {
	assert, _ := makeAR(t)

	var entry fib.Entry
	c := (*C.FibEntry)(unsafe.Pointer(&entry))
	ndntestenv.NameEqual(assert, "/", &entry)
	assert.EqualValues(0, c.nComps)
	assert.Len(entry.Nexthops(), 0)

	name := ndn.ParseName("/A/B")
	assert.NoError(entry.SetName(name))
	ndntestenv.NameEqual(assert, name, &entry)
	assert.EqualValues(2, c.nComps)

	nexthops := []iface.ID{2302, 1067, 1122}
	assert.NoError(entry.SetNexthops(nexthops))
	assert.Equal(nexthops, entry.Nexthops())

	name2V, _ := name.MarshalBinary()
	for len(name2V) <= fib.MaxNameLength {
		name2V = append(name2V, name2V...)
	}

	var name2 ndn.Name
	name2.UnmarshalBinary(name2V)
	assert.Error(entry.SetName(name2))

	nexthops2 := make([]iface.ID, 0)
	for len(nexthops2) <= fib.MaxNexthops {
		nexthops2 = append(nexthops2, iface.ID(5000+len(nexthops2)))
	}
	assert.Error(entry.SetNexthops(nexthops2))
}
