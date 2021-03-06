package main

import (
	"os"

	"github.com/usnistgov/ndn-dpdk/app/fwdp"
	"github.com/usnistgov/ndn-dpdk/container/fib"
	"github.com/usnistgov/ndn-dpdk/container/ndt"
	"github.com/usnistgov/ndn-dpdk/core/gqlserver"
	"github.com/usnistgov/ndn-dpdk/dpdk/ealinit"
	"github.com/usnistgov/ndn-dpdk/dpdk/ealthread"
	"github.com/usnistgov/ndn-dpdk/mgmt/hrlog"
)

var dp *fwdp.DataPlane

func main() {
	gqlserver.Start()

	initCfg, e := parseCommand(ealinit.Init(os.Args)[1:])
	if e != nil {
		log.WithError(e).Fatal("command line error")
	}
	hrlog.Init()

	initCfg.Mempool.Apply()
	ealthread.DefaultAllocator.Config = initCfg.LCoreAlloc

	startDp(initCfg.Ndt, initCfg.Fib, initCfg.Fwdp)
	startMgmt()

	select {}
}

func startDp(ndtCfg ndt.Config, fibCfg fib.Config, dpInit fwdpInitConfig) {
	var dpCfg fwdp.Config
	dpCfg.Ndt = ndtCfg
	dpCfg.Fib = fibCfg
	dpCfg.Suppress = dpInit.Suppress

	// set crypto config
	dpCfg.Crypto.InputCapacity = 64
	dpCfg.Crypto.OpPoolCapacity = 1023

	// set dataplane config
	dpCfg.FwdInterestQueue = dpInit.FwdInterestQueue
	dpCfg.FwdDataQueue = dpInit.FwdDataQueue
	dpCfg.FwdNackQueue = dpInit.FwdNackQueue
	dpCfg.LatencySampleFreq = dpInit.LatencySampleFreq
	dpCfg.Pcct.MaxEntries = dpInit.PcctCapacity
	dpCfg.Pcct.CsCapMd = dpInit.CsCapMd
	dpCfg.Pcct.CsCapMi = dpInit.CsCapMi

	// create and launch dataplane
	var e error
	dp, e = fwdp.New(dpCfg)
	if e != nil {
		log.WithError(e).Fatal("dataplane init error")
	}

	log.Info("dataplane started")
}
