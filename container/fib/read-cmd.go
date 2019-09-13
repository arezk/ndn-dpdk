package fib

/*
#include "fib.h"
*/
import "C"
import (
	"ndn-dpdk/core/urcu"
	"ndn-dpdk/ndn"
)

// List all FIB entry names.
func (fib *Fib) ListNames() (names []*ndn.Name) {
	fib.postCommand(func(rs *urcu.ReadSide) error {
		names = make([]*ndn.Name, 0)
		fib.treeRoot.Walk(nodeName{}, func(nn nodeName, node *node) {
			if node.IsEntry {
				names = append(names, nn.GetName())
			}
		})
		return nil
	})
	return names
}

func findC(fibC *C.Fib, nameV ndn.TlvBytes, hash uint64) (entryC *C.FibEntry) {
	return C.Fib_Find_(fibC, C.uint16_t(len(nameV)), (*C.uint8_t)(nameV.GetPtr()),
		C.uint64_t(hash))
}

// Perform an exact match lookup.
func (fib *Fib) Find(name *ndn.Name) (entry *Entry) {
	fib.postCommand(func(rs *urcu.ReadSide) error {
		_, partition := fib.ndt.Lookup(name)
		entry = fib.FindInPartition(name, int(partition), rs)
		return nil
	})
	return entry
}

// Perform an exact match lookup in specified partition.
// This method runs in the given URCU read-side thread, not necessarily the command loop.
func (fib *Fib) FindInPartition(name *ndn.Name, partition int, rs *urcu.ReadSide) (entry *Entry) {
	rs.Lock()
	defer rs.Unlock()
	entryC := findC(fib.c[partition], name.GetValue(), name.ComputeHash())
	if entryC != nil {
		entry = &Entry{*entryC}
	}
	return entry
}

// Read entry counters, aggregate across all partitions if necessary.
func (fib *Fib) ReadEntryCounters(name *ndn.Name) (cnt EntryCounters) {
	fib.postCommand(func(rs *urcu.ReadSide) error {
		if name.Len() < fib.ndt.GetPrefixLen() {
			for partition := 0; partition < len(fib.c); partition++ {
				if entry := fib.FindInPartition(name, partition, rs); entry != nil {
					cnt.Add(entry)
				}
			}
		} else {
			_, partition := fib.ndt.Lookup(name)
			if entry := fib.FindInPartition(name, int(partition), rs); entry != nil {
				cnt.Add(entry)
			}
		}
		return nil
	})
	return cnt
}

// Perform a longest prefix match lookup.
func (fib *Fib) Lpm(name *ndn.Name) (entry *Entry) {
	fib.postCommand(func(rs *urcu.ReadSide) error {
		rs.Lock()
		defer rs.Unlock()
		_, partition := fib.ndt.Lookup(name)
		entryC := C.Fib_Lpm_(fib.c[partition], (*C.PName)(name.GetPNamePtr()),
			(*C.uint8_t)(name.GetValue().GetPtr()))
		if entryC != nil {
			entry = &Entry{*entryC}
		}
		return nil
	})
	return entry
}
