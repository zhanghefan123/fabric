/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package smartbft

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/hyperledger-labs/SmartBFT/pkg/types"
	"github.com/hyperledger/fabric-lib-go/bccsp"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric/common/deliverclient"
	"github.com/hyperledger/fabric/common/deliverclient/blocksprovider"
	"github.com/hyperledger/fabric/common/deliverclient/orderers"
	"github.com/hyperledger/fabric/orderer/common/cluster"
	"github.com/hyperledger/fabric/orderer/common/localconfig"
	"github.com/hyperledger/fabric/orderer/consensus"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
)

type BFTSynchronizer struct {
	lastReconfig        types.Reconfig
	selfID              uint64
	LatestConfig        func() (types.Configuration, []uint64)
	BlockToDecision     func(*common.Block) *types.Decision
	OnCommit            func(*common.Block) types.Reconfig
	Support             consensus.ConsenterSupport
	CryptoProvider      bccsp.BCCSP
	ClusterDialer       *cluster.PredicateDialer
	LocalConfigCluster  localconfig.Cluster
	BlockPullerFactory  BlockPullerFactory
	VerifierFactory     VerifierFactory
	BFTDelivererFactory BFTDelivererFactory
	Logger              *flogging.FabricLogger

	mutex    sync.Mutex
	syncBuff *SyncBuffer
}

func (s *BFTSynchronizer) Sync() types.SyncResponse {
	s.Logger.Debug("BFT Sync initiated")
	decision, err := s.synchronize()
	if err != nil {
		s.Logger.Warnf("Could not synchronize with remote orderers due to %s, returning state from local ledger", err)
		block := s.Support.Block(s.Support.Height() - 1)
		config, nodes := s.LatestConfig()
		return types.SyncResponse{
			Latest: *s.BlockToDecision(block),
			Reconfig: types.ReconfigSync{
				InReplicatedDecisions: false, // If we read from ledger we do not need to reconfigure.
				CurrentNodes:          nodes,
				CurrentConfig:         config,
			},
		}
	}

	// After sync has ended, reset the state of the last reconfig.
	defer func() {
		s.lastReconfig = types.Reconfig{}
	}()

	s.Logger.Debugf("reconfig: %+v", s.lastReconfig)
	return types.SyncResponse{
		Latest: *decision,
		Reconfig: types.ReconfigSync{
			InReplicatedDecisions: s.lastReconfig.InLatestDecision,
			CurrentConfig:         s.lastReconfig.CurrentConfig,
			CurrentNodes:          s.lastReconfig.CurrentNodes,
		},
	}
}

// Buffer return the internal SyncBuffer for testability.
func (s *BFTSynchronizer) Buffer() *SyncBuffer {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.syncBuff
}

// MaliciousSynchronize 恶意的同步
func (s *BFTSynchronizer) MaliciousSynchronize() error {
	//// === We probe all the endpoints and establish a target height, as well as detect the self endpoint.
	//targetHeight, myEndpoint, err := s.detectTargetHeight()
	//if err != nil {
	//	return errors.Wrapf(err, "cannot get detect target height")
	//}
	//
	//var fasifiedHeight uint64 = 0
	//// === Create a buffer to accept the blocks delivered from the BFTDeliverer.
	//// 计算能够存储的最大的能够缓存的区块的数量
	//capacityBlocks := uint(s.LocalConfigCluster.ReplicationBufferSize) / uint(s.Support.SharedConfig().BatchSize().AbsoluteMaxBytes)
	//// 如果能够缓存的区块数量 < 100, 那么至少设置为 100
	//if capacityBlocks < 100 {
	//	capacityBlocks = 100
	//}
	//s.mutex.Lock()
	//s.syncBuff = NewSyncBuffer(capacityBlocks)
	//s.mutex.Unlock()
	//
	//// === Create the BFT block deliverer and start a go-routine that fetches block and inserts them into the syncBuffer.
	//// 创建 BFT block deliver 并且启动一个 goroutine 来将获取到的 blocks 放到  syncBuffer 之中
	//// 假设在 startHeight 处每次填写0的话, 然后每次向主节点进行区块同步
	//bftDeliverer, err := s.createBFTDeliverer(fasifiedHeight, myEndpoint)
	//if err != nil {
	//	return errors.Wrapf(err, "cannot create BFT block deliverer")
	//}
	//
	//go bftDeliverer.DeliverBlocks()
	//defer bftDeliverer.Stop()
	//
	//// === Loop on sync-buffer and pull blocks, writing them to the ledger, returning the last block pulled.
	//lastPulledBlock, err := s.getBlocksFromSyncBufferMalicious(fasifiedHeight, targetHeight)
	//if err != nil {
	//	return errors.Wrapf(err, "failed to get any blocks from SyncBuffer")
	//}
	//
	//// zhf add code 打印最后拉取的区块
	//fmt.Printf("last pulled block's heigth = %d\n", lastPulledBlock.Header.Number)
	return nil
}

func (s *BFTSynchronizer) synchronize() (*types.Decision, error) {
	fmt.Println("zhf add code: synchronize")

	// === We probe all the endpoints and establish a target height, as well as detect the self endpoint.
	targetHeight, myEndpoint, err := s.detectTargetHeight()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get detect target height")
	}

	// 获取当前的高度
	startHeight := s.Support.Height()
	// 如果当前的高度 >= 其他节点告知的高度
	if startHeight >= targetHeight {
		// 返回错误
		return nil, errors.Errorf("already at target height of %d", targetHeight)
	}

	// === Create a buffer to accept the blocks delivered from the BFTDeliverer.
	// 计算能够存储的最大的能够缓存的区块的数量
	capacityBlocks := uint(s.LocalConfigCluster.ReplicationBufferSize) / uint(s.Support.SharedConfig().BatchSize().AbsoluteMaxBytes)
	// 如果能够缓存的区块数量 < 100, 那么至少设置为 100
	if capacityBlocks < 100 {
		capacityBlocks = 100
	}
	s.mutex.Lock()
	s.syncBuff = NewSyncBuffer(capacityBlocks)
	s.mutex.Unlock()

	// === Create the BFT block deliverer and start a go-routine that fetches block and inserts them into the syncBuffer.
	// 创建 BFT block deliver 并且启动一个 goroutine 来将获取到的 blocks 放到  syncBuffer 之中
	// 假设在 startHeight 处每次填写0的话, 然后每次向主节点进行区块同步
	//fmt.Println("zhf add code: createBFTDeliverer")
	bftDeliverer, err := s.createBFTDeliverer(startHeight, myEndpoint)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create BFT block deliverer")
	}

	go bftDeliverer.DeliverBlocks()
	defer bftDeliverer.Stop()

	// === Loop on sync-buffer and pull blocks, writing them to the ledger, returning the last block pulled.
	lastPulledBlock, err := s.getBlocksFromSyncBuffer(startHeight, targetHeight)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get any blocks from SyncBuffer")
	}

	// 拿到最后一个同步过来的区块
	decision := s.BlockToDecision(lastPulledBlock)
	s.Logger.Infof("Returning decision from block [%d], decision: %+v", lastPulledBlock.GetHeader().GetNumber(), decision)
	return decision, nil
}

// detectTargetHeight probes remote endpoints and detects what is the target height this node needs to reach. It also
// detects the self-endpoint.
//
// In BFT it is highly recommended that the channel/orderer-endpoints (for delivery & broadcast) map 1:1 to the
// channel/orderers/consenters (for cluster consensus), that is, every consenter should be represented by a
// delivery endpoint. This important for Sync to work properly.
func (s *BFTSynchronizer) detectTargetHeight() (uint64, string, error) {
	blockPuller, err := s.BlockPullerFactory.CreateBlockPuller(s.Support, s.ClusterDialer, s.LocalConfigCluster, s.CryptoProvider)
	if err != nil {
		return 0, "", errors.Wrap(err, "cannot get create BlockPuller")
	}
	defer blockPuller.Close()

	// 拿到各个验证者的 height
	heightByEndpoint, myEndpoint, err := blockPuller.HeightsByEndpoints()
	if err != nil {
		return 0, "", errors.Wrap(err, "cannot get HeightsByEndpoints")
	}

	s.Logger.Infof("HeightsByEndpoints: %+v, my endpoint: %s", heightByEndpoint, myEndpoint)

	delete(heightByEndpoint, myEndpoint)
	var heights []uint64
	for _, value := range heightByEndpoint {
		heights = append(heights, value)
	}

	if len(heights) == 0 {
		return 0, "", errors.New("no cluster members to synchronize with")
	}

	// 计算拿到的正确的高度
	targetHeight := s.computeTargetHeight(heights)
	s.Logger.Infof("Detected target height: %d", targetHeight)
	return targetHeight, myEndpoint, nil
}

// computeTargetHeight compute the target height to synchronize to.
// heights: a slice containing the heights of accessible peers, length must be >0.
// clusterSize: the cluster size, must be >0.
func (s *BFTSynchronizer) computeTargetHeight(heights []uint64) uint64 {
	// 将区块按照降序进行排列
	sort.Slice(heights, func(i, j int) bool { return heights[i] > heights[j] }) // Descending
	// 获取集群的大小
	clusterSize := len(s.Support.SharedConfig().Consenters())
	// 获取容忍的拜占庭节点的数量
	f := uint64(clusterSize-1) / 3 // The number of tolerated byzantine faults
	lenH := uint64(len(heights))

	s.Logger.Debugf("Cluster size: %d, F: %d, Heights: %v", clusterSize, f, heights)

	// 如果高度数量不足 (f+1), 返回最小的那个高度
	if lenH < f+1 {
		s.Logger.Debugf("Returning %d", heights[0])
		return heights[int(lenH)-1]
	}
	s.Logger.Debugf("Returning %d", heights[f])
	// 如果高度足 f + 1, 那么选择第 f + 1 个高度，可以确保至少有 f+1 个节点报告了该高度或更高高度
	return heights[f]
}

// createBFTDeliverer creates and initializes the BFT block deliverer.
func (s *BFTSynchronizer) createBFTDeliverer(startHeight uint64, myEndpoint string) (BFTBlockDeliverer, error) {
	lastBlock := s.Support.Block(startHeight - 1)
	lastConfigBlock, err := cluster.LastConfigBlock(lastBlock, s.Support)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve last config block")
	}
	lastConfigEnv, err := deliverclient.ConfigFromBlock(lastConfigBlock)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve last config envelope")
	}

	var updatableVerifier deliverclient.CloneableUpdatableBlockVerifier
	updatableVerifier, err = s.VerifierFactory.CreateBlockVerifier(lastConfigBlock, lastBlock, s.CryptoProvider, s.Logger)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create BlockVerificationAssistant")
	}

	clientConfig := s.ClusterDialer.Config // The cluster and block puller use slightly different options
	clientConfig.AsyncConnect = false
	clientConfig.SecOpts.VerifyCertificate = nil

	// The maximal amount of time to wait before retrying to connect.
	maxRetryInterval := s.LocalConfigCluster.ReplicationRetryTimeout
	// The minimal amount of time to wait before retrying. The retry interval doubles after every unsuccessful attempt.
	minRetryInterval := maxRetryInterval / 50
	// The maximal duration of a Sync. After this time Sync returns with whatever it had pulled until that point.
	maxRetryDuration := s.LocalConfigCluster.ReplicationPullTimeout * time.Duration(s.LocalConfigCluster.ReplicationMaxRetries)
	// If a remote orderer does not deliver blocks for this amount of time, even though it can do so, it is replaced as the block deliverer.
	blockCesorshipTimeOut := maxRetryDuration / 3

	bftDeliverer := s.BFTDelivererFactory.CreateBFTDeliverer(
		s.Support.ChannelID(),
		s.syncBuff,
		&ledgerInfoAdapter{s.Support},
		updatableVerifier,
		blocksprovider.DialerAdapter{ClientConfig: clientConfig},
		&orderers.ConnectionSourceFactory{}, // no overrides in the orderer
		s.CryptoProvider,
		make(chan struct{}),
		s.Support,
		blocksprovider.DeliverAdapter{},
		&blocksprovider.BFTCensorshipMonitorFactory{},
		flogging.MustGetLogger("orderer.blocksprovider").With("channel", s.Support.ChannelID()),
		minRetryInterval,
		maxRetryInterval,
		blockCesorshipTimeOut,
		maxRetryDuration,
		func() (stopRetries bool) {
			s.syncBuff.Stop()
			return true // In the orderer we must limit the time we try to do Synch()
		},
	)

	s.Logger.Infof("Created a BFTDeliverer: %+v", bftDeliverer)
	bftDeliverer.Initialize(lastConfigEnv.GetConfig(), myEndpoint)

	return bftDeliverer, nil
}

// zhf add code getBlocksFromSyncBufferMalicious 在恶意攻击的时候不用对区块进行任何处理
func (s *BFTSynchronizer) getBlocksFromSyncBufferMalicious(startHeight, targetHeight uint64) (*common.Block, error) {
	targetSeq := targetHeight - 1
	seq := startHeight
	var blocksFetched int
	var lastPulledBlock *common.Block
	for seq <= targetSeq {
		// 从 syncBuffer 之中取出区块
		block := s.syncBuff.PullBlock(seq)
		lastPulledBlock = block
		seq++
		blocksFetched++
	}
	s.syncBuff.Stop()
	return lastPulledBlock, nil
}

func (s *BFTSynchronizer) getBlocksFromSyncBuffer(startHeight, targetHeight uint64) (*common.Block, error) {
	targetSeq := targetHeight - 1
	seq := startHeight
	var blocksFetched int
	s.Logger.Debugf("Will fetch sequences [%d-%d]", seq, targetSeq)

	var lastPulledBlock *common.Block
	for seq <= targetSeq {
		// 从 syncBuffer 之中取出区块
		block := s.syncBuff.PullBlock(seq)
		if block == nil {
			s.Logger.Debugf("Failed to fetch block [%d] from cluster", seq)
			break
		}
		// 如果是配置块
		if protoutil.IsConfigBlock(block) {
			s.Support.WriteConfigBlock(block, nil)
			s.Logger.Debugf("Fetched and committed config block [%d] from cluster", seq)
		} else {
			// 将区块写入到账本之中, 因为网络分区原因, 最后同步过来的账本验证完成后直接写入
			s.Support.WriteBlock(block, nil)
			s.Logger.Debugf("Fetched and committed block [%d] from cluster", seq)
		}
		lastPulledBlock = block

		prevInLatestDecision := s.lastReconfig.InLatestDecision
		s.lastReconfig = s.OnCommit(lastPulledBlock)
		s.lastReconfig.InLatestDecision = s.lastReconfig.InLatestDecision || prevInLatestDecision
		s.Logger.Debugf("Last reconfig %+v", s.lastReconfig)
		seq++
		blocksFetched++
	}

	s.syncBuff.Stop()

	if lastPulledBlock == nil {
		return nil, errors.Errorf("failed pulling block %d", seq)
	}

	startSeq := startHeight
	s.Logger.Infof("Finished synchronizing with cluster, fetched %d blocks, starting from block [%d], up until and including block [%d]",
		blocksFetched, startSeq, lastPulledBlock.Header.Number)

	return lastPulledBlock, nil
}
