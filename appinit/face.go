package appinit

import (
	"fmt"
	"net"

	"ndn-dpdk/dpdk"
	"ndn-dpdk/iface"
	"ndn-dpdk/iface/ethface"
	"ndn-dpdk/iface/faceuri"
	"ndn-dpdk/iface/socketface"
)

var theFaceTable iface.FaceTable

func GetFaceTable() iface.FaceTable {
	if theFaceTable.GetPtr() == nil {
		theFaceTable = iface.NewFaceTable()
	}
	return theFaceTable
}

var FACE_RXQ_CAPACITY = 64 // RX queue capacity for new faces
var FACE_TXQ_CAPACITY = 64 // TX queue capacity for new faces

func NewFaceFromUri(u faceuri.FaceUri) (*iface.Face, error) {
	create := newFaceByScheme[u.Scheme]
	if create == nil {
		return nil, fmt.Errorf("cannot create face with scheme %s", u.Scheme)
	}
	return create(u)
}

var newFaceByScheme = map[string]func(u faceuri.FaceUri) (*iface.Face, error){
	"dev":  newEthFace,
	"udp4": newSocketFace,
	"tcp4": newSocketFace,
}

func newEthFace(u faceuri.FaceUri) (*iface.Face, error) {
	port := dpdk.FindEthDev(u.Host)
	if !port.IsValid() {
		return nil, fmt.Errorf("DPDK device %s not found", u.Host)
	}

	var cfg dpdk.EthDevConfig
	cfg.AddRxQueue(dpdk.EthRxQueueConfig{Capacity: FACE_RXQ_CAPACITY,
		Socket: port.GetNumaSocket(),
		Mp:     MakePktmbufPool(MP_ETHRX, port.GetNumaSocket())})
	cfg.AddTxQueue(dpdk.EthTxQueueConfig{Capacity: FACE_TXQ_CAPACITY,
		Socket: port.GetNumaSocket()})
	_, _, e := port.Configure(cfg)
	if e != nil {
		return nil, fmt.Errorf("port(%d).Configure: %v", port, e)
	}

	port.SetPromiscuous(true)

	e = port.Start()
	if e != nil {
		return nil, fmt.Errorf("port(%d).Start: %v", port, e)
	}

	face, e := ethface.New(port, makeFaceMempools(port.GetNumaSocket()))
	if e != nil {
		return nil, fmt.Errorf("ethface.New(%d): %v", port, e)
	}

	GetFaceTable().SetFace(face.Face)
	return &face.Face, nil
}

func newSocketFace(u faceuri.FaceUri) (*iface.Face, error) {
	network, address := u.Scheme[:3], u.Host

	conn, e := net.Dial(network, address)
	if e != nil {
		return nil, fmt.Errorf("net.Dial(%s,%s): %v", network, address, e)
	}

	var cfg socketface.Config
	cfg.Mempools = makeFaceMempools(dpdk.NUMA_SOCKET_ANY)
	cfg.RxMp = MakePktmbufPool(MP_ETHRX, dpdk.NUMA_SOCKET_ANY)
	cfg.RxqCapacity = FACE_RXQ_CAPACITY
	cfg.TxqCapacity = FACE_TXQ_CAPACITY

	face := socketface.New(conn, cfg)
	GetFaceTable().SetFace(face.Face)
	return &face.Face, nil
}

func makeFaceMempools(socket dpdk.NumaSocket) (mempools iface.Mempools) {
	mempools.IndirectMp = MakePktmbufPool(MP_IND, socket)
	mempools.NameMp = MakePktmbufPool(MP_NAME, socket)
	mempools.HeaderMp = MakePktmbufPool(MP_ETHTX, socket)
	return mempools
}

func MakeRxLooper(face iface.Face) iface.IRxLooper {
	faceId := face.GetFaceId()
	switch faceId.GetKind() {
	case iface.FaceKind_EthDev:
		return ethface.EthFace{face}
	case iface.FaceKind_Socket:
		return socketface.NewRxGroup(socketface.Get(faceId))
	}
	return nil
}
