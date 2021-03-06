#ifndef NDNDPDK_IFACE_TX_PROC_H
#define NDNDPDK_IFACE_TX_PROC_H

/** @file */

#include "../core/running-stat.h"
#include "common.h"

typedef struct TxProc TxProc;

typedef uint16_t (*TxProc_OutputFunc_)(TxProc* tx, Packet* npkt, struct rte_mbuf** frames);

/** @brief Outgoing packet processing procedure. */
typedef struct TxProc
{
  struct rte_mempool* indirectMp;
  struct rte_mempool* headerMp;
  TxProc_OutputFunc_ outputFunc;

  uint32_t fragmentPayloadSize; ///< max payload size per fragment
  uint16_t headerHeadroom;      ///< headroom for header mbuf

  uint64_t nextSeqNum; ///< next fragmentation sequence number

  uint64_t nL3Fragmented; ///< L3 packets that required fragmentation
  uint64_t nL3OverLength; ///< dropped L3 packets due to over length
  uint64_t nAllocFails;   ///< dropped L3 packets due to allocation failure

  uint64_t nFrames;        ///< sent+dropped L2 frames
  uint64_t nOctets;        ///< sent+dropped L2 octets (including LpHeader)
  uint64_t nDroppedFrames; ///< dropped L2 frames
  uint64_t nDroppedOctets; ///< dropped L2 octets

  /**
   * @brief Statistics of L3 latency, per L3 packet type.
   *
   * Latency counting starts from packet arrival or generation, and ends when
   * packet is queuing for transmission; this counts per L3 packet.
   * This is taken before fragmentation, so that it includes packets dropped due to full queue.
   */
  RunningStat latency[PktMax];
} __rte_cache_aligned TxProc;

/**
 * @brief Initialize TX procedure.
 * @param mtu transport MTU available for NDNLP packets.
 * @param headroom headroom before NDNLP header, as required by transport.
 * @param indirectMp mempool for indirect mbufs.
 * @param headerMp mempool for NDNLP headers; must have
 *                 headroom + LpHeaderHeadroom dataroom.
 */
__attribute__((nonnull)) void
TxProc_Init(TxProc* tx, uint16_t mtu, uint16_t headroom, struct rte_mempool* indirectMp,
            struct rte_mempool* headerMp);

/**
 * @brief Process an outgoing L3 packet.
 * @param npkt outgoing L3 packet; TxProc takes ownership.
 * @param[out] frames L2 frames to be transmitted; TxProc releases ownership.
 * @return number of L2 frames to be transmitted.
 */
__attribute__((nonnull)) static inline uint16_t
TxProc_Output(TxProc* tx, Packet* npkt, struct rte_mbuf* frames[LpMaxFragments])
{
  return (*tx->outputFunc)(tx, npkt, frames);
}

#endif // NDNDPDK_IFACE_TX_PROC_H
