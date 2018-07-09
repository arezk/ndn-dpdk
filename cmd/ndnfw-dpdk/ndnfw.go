package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"ndn-dpdk/app/fwdp"
	"ndn-dpdk/app/timing"
	"ndn-dpdk/appinit"
	"ndn-dpdk/container/fib"
	"ndn-dpdk/container/ndt"
	"ndn-dpdk/container/ndt/ndtupdater"
	"ndn-dpdk/dpdk"
	"ndn-dpdk/iface"
	"ndn-dpdk/iface/socketface"
	"ndn-dpdk/mgmt/facemgmt"
	"ndn-dpdk/mgmt/fibmgmt"
	"ndn-dpdk/mgmt/fwdpmgmt"
	"ndn-dpdk/mgmt/ndtmgmt"
	"ndn-dpdk/mgmt/versionmgmt"
	"ndn-dpdk/strategy/strategy_elf"
)

var (
	theSocketFaceNumaSocket dpdk.NumaSocket
	theSocketRxg            *socketface.RxGroup
	theSocketTxl            *iface.MultiTxLoop
	theNdt                  *ndt.Ndt
	theStrategy             fib.StrategyCode
	theFib                  *fib.Fib
	theDp                   *fwdp.DataPlane
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	appinit.InitEal()
	initCfg, e := parseCommand(appinit.Eal.Args[1:])
	if e != nil {
		log.WithError(e).Fatal("command line error")
	}
	initCfg.Mempool.Apply()
	initCfg.FaceQueueCapacity.Apply()

	var timingRingCapacity int
	var tw timing.Writer
	fmt.Sscanf(os.Getenv("TIMING_WRITER"), "%d:%d:%d:%s", &timingRingCapacity,
		&tw.NTotal, &tw.NSkip, &tw.Filename)
	timing.Init(timingRingCapacity)

	startDp(initCfg.Ndt, initCfg.Fib, initCfg.Fwdp)
	theStrategy = loadStrategy("multicast")
	theStrategy.Ref()
	startMgmt()

	if tw.Filename != "" {
		tw.Run()
	} else {
		select {}
	}
}

func startDp(ndtCfg ndt.Config, fibCfg fib.Config, dpInit fwdpInitConfig) {
	log.WithField("nSlaves", len(appinit.Eal.Slaves)).Info("EAL ready")
	lcr := appinit.NewLCoreReservations()

	var dpCfg fwdp.Config

	// reserve lcores for EthFace
	var inputRxLoopers []iface.IRxLooper
	var outputLCores []dpdk.LCore
	var outputTxLoopers []iface.ITxLooper
	for _, port := range dpdk.ListEthDevs() {
		logEntry := log.WithFields(makeLogFields("port", port, "name", port.GetName()))
		face, e := appinit.NewFaceFromEthDev(port)
		if e != nil {
			logEntry.WithError(e).Fatal("EthFace creation error")
			continue
		}
		inputLc := lcr.MustReserve(face.GetNumaSocket())
		socket := inputLc.GetNumaSocket()
		logEntry = logEntry.WithFields(makeLogFields("face", face.GetFaceId(), "rx-lcore", inputLc, "socket", socket))
		dpCfg.InputLCores = append(dpCfg.InputLCores, inputLc)
		inputRxLoopers = append(inputRxLoopers, appinit.MakeRxLooper(face))

		e = face.EnableThreadSafeTx(appinit.TheFaceQueueCapacityConfig.EthTxPkts)
		if e != nil {
			logEntry.WithError(e).Fatal("EnableThreadSafeTx failed")
		}

		outputLc := lcr.MustReserve(socket)
		logEntry.WithField("tx-lcore", outputLc).Info("EthFace created")
		outputLCores = append(outputLCores, outputLc)
		outputTxLoopers = append(outputTxLoopers, appinit.MakeTxLooper(face))
	}

	// reserve lcore for SocketFaces
	{
		theSocketRxg = socketface.NewRxGroup()
		inputLc := lcr.MustReserve(dpdk.NUMA_SOCKET_ANY)
		theSocketFaceNumaSocket = inputLc.GetNumaSocket()
		dpCfg.InputLCores = append(dpCfg.InputLCores, inputLc)
		inputRxLoopers = append(inputRxLoopers, theSocketRxg)

		theSocketTxl = iface.NewMultiTxLoop()
		outputLc := lcr.MustReserve(theSocketFaceNumaSocket)
		outputLCores = append(outputLCores, outputLc)
		outputTxLoopers = append(outputTxLoopers, theSocketTxl)

		log.WithFields(makeLogFields("rx-lcore", inputLc, "socket", theSocketFaceNumaSocket,
			"tx-lcore", outputLc)).Info("SocketFaces ready")
	}

	// reserve lcores for forwarding processes
	nFwds := len(appinit.Eal.Slaves) - len(dpCfg.InputLCores) - len(outputLCores)
	for len(dpCfg.FwdLCores) < nFwds {
		lc := lcr.Reserve(dpdk.NUMA_SOCKET_ANY)
		if !lc.IsValid() {
			break
		}
		log.WithFields(makeLogFields("lcore", lc, "socket", lc.GetNumaSocket())).Info("fwd created")
		dpCfg.FwdLCores = append(dpCfg.FwdLCores, lc)
	}
	nFwds = len(dpCfg.FwdLCores)
	if nFwds <= 0 {
		log.Fatal("no lcore available for forwarding")
	}

	// initialize NDT
	{
		theNdt = ndt.New(ndtCfg, dpdk.ListNumaSocketsOfLCores(dpCfg.InputLCores))
		dpCfg.Ndt = theNdt
	}

	// initialize FIB
	{
		fibCfg.Id = "FIB"
		var e error
		theFib, e = fib.New(fibCfg, theNdt, dpdk.ListNumaSocketsOfLCores(dpCfg.FwdLCores))
		if e != nil {
			log.WithError(e).Fatal("FIB creation failed")
		}
		dpCfg.Fib = theFib
	}

	// randomize NDT
	theNdt.Randomize(nFwds)

	// set forwarding process config
	dpCfg.FwdQueueCapacity = dpInit.FwdQueueCapacity
	dpCfg.LatencySampleFreq = dpInit.LatencySampleFreq
	dpCfg.PcctCfg.MaxEntries = dpInit.PcctCapacity
	dpCfg.CsCapacity = dpInit.CsCapacity

	// create dataplane
	{
		var e error
		theDp, e = fwdp.New(dpCfg)
		if e != nil {
			log.WithError(e).Fatal("dataplane init error")
		}
	}

	// launch output lcores
	log.Info("launching output lcores")
	for i := range outputTxLoopers {
		func(i int) {
			outputLCores[i].RemoteLaunch(func() int {
				txl := outputTxLoopers[i]
				txl.TxLoop()
				return 0
			})
		}(i)
	}

	// launch forwarding lcores
	log.Info("launching forwarding lcores")
	for i := range dpCfg.FwdLCores {
		e := theDp.LaunchFwd(i)
		if e != nil {
			log.WithError(e).WithField("i", i).Fatal("fwd launch failed")
		}
	}

	// launch input lcores
	log.Info("launching input lcores")
	const burstSize = 64
	for i, rxl := range inputRxLoopers {
		e := theDp.LaunchInput(i, rxl, burstSize)
		if e != nil {
			log.WithError(e).WithField("i", i).Fatal("input launch failed")
		}
	}

	log.Info("dataplane started")
}

func startMgmt() {
	appinit.RegisterMgmt(versionmgmt.VersionMgmt{})

	facemgmt.CreateFace = socketface.MakeMgmtCreateFace(
		appinit.NewSocketFaceCfg(theSocketFaceNumaSocket), theSocketRxg, theSocketTxl,
		appinit.TheFaceQueueCapacityConfig.SocketTxPkts)
	appinit.RegisterMgmt(facemgmt.FaceMgmt{})

	appinit.RegisterMgmt(ndtmgmt.NdtMgmt{
		Ndt: theNdt,
		Updater: &ndtupdater.NdtUpdater{
			Ndt:      theNdt,
			Fib:      theFib,
			SleepFor: 200 * time.Millisecond,
		},
	})

	fibmgmt.TheStrategy = theStrategy
	appinit.RegisterMgmt(fibmgmt.FibMgmt{theFib})

	appinit.RegisterMgmt(fwdpmgmt.DpInfoMgmt{theDp})

	appinit.StartMgmt()
}

func loadStrategy(shortname string) fib.StrategyCode {
	logEntry := log.WithField("strategy", shortname)

	elf, e := strategy_elf.Load(shortname)
	if e != nil {
		logEntry.WithError(e).Fatal("strategy ELF load error")
	}
	sc, e := theFib.LoadStrategyCode(elf)
	if e != nil {
		logEntry.WithError(e).Fatal("strategy code load error")
	}

	logEntry.Debug("strategy loaded")
	return sc
}
