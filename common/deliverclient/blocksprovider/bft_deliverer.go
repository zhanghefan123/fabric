/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blocksprovider

import (
	"fmt"
	"github.com/hyperledger/fabric/zeusnet/bft_related"
	"github.com/hyperledger/fabric/zeusnet/modules/config"
	"sync"
	"time"

	"github.com/hyperledger/fabric-lib-go/bccsp"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"github.com/hyperledger/fabric/common/deliverclient"
	"github.com/hyperledger/fabric/common/deliverclient/orderers"
	"github.com/hyperledger/fabric/internal/pkg/identity"
	"github.com/pkg/errors"
)

//go:generate counterfeiter -o fake/censorship_detector.go --fake-name CensorshipDetector . CensorshipDetector
type CensorshipDetector interface {
	Monitor()
	Stop()
	ErrorsChannel() <-chan error
}

//go:generate counterfeiter -o fake/censorship_detector_factory.go --fake-name CensorshipDetectorFactory . CensorshipDetectorFactory
type CensorshipDetectorFactory interface {
	Create(
		chainID string,
		updatableVerifier UpdatableBlockVerifier,
		requester DeliverClientRequester,
		progressReporter BlockProgressReporter,
		fetchSources []*orderers.Endpoint,
		blockSourceIndex int,
		timeoutConf TimeoutConfig,
	) CensorshipDetector
}

//go:generate counterfeiter -o fake/duration_exceeded_handler.go --fake-name DurationExceededHandler . DurationExceededHandler
type DurationExceededHandler interface {
	DurationExceededHandler() (stopRetries bool)
}

//go:generate counterfeiter -o fake/updatable_block_verifier.go --fake-name UpdatableBlockVerifier . UpdatableBlockVerifier
type UpdatableBlockVerifier deliverclient.CloneableUpdatableBlockVerifier

// BFTDeliverer fetches blocks using a block receiver and maintains a BFTCensorshipMonitor.
// It maintains a shuffled orderer source slice, and will cycle through it trying to find a "good" orderer to fetch
// blocks from. After it selects an orderer to fetch blocks from, it assigns all the rest of the orderers to the
// censorship monitor. The censorship monitor will request block attestations (header+sigs) from said orderers, and
// will monitor their progress relative to the block fetcher. If a censorship suspicion is detected, the BFTDeliverer
// will try to find another orderer to fetch from.
type BFTDeliverer struct {
	ChannelID    string
	BlockHandler BlockHandler
	Ledger       LedgerInfo

	UpdatableBlockVerifier    UpdatableBlockVerifier
	Dialer                    Dialer
	OrderersSourceFactory     OrdererConnectionSourceFactory
	CryptoProvider            bccsp.BCCSP
	DoneC                     chan struct{}
	Signer                    identity.SignerSerializer
	DeliverStreamer           DeliverStreamer
	CensorshipDetectorFactory CensorshipDetectorFactory
	Logger                    *flogging.FabricLogger

	// The initial value of the actual retry interval, which is increased on every failed retry
	InitialRetryInterval time.Duration
	// The maximal value of the actual retry interval, which cannot increase beyond this value
	MaxRetryInterval time.Duration
	// If a certain header from a header receiver is in front of the block receiver for more that this time, a
	// censorship event is declared and the block source is changed.
	BlockCensorshipTimeout time.Duration
	// After this duration, the MaxRetryDurationExceededHandler is called to decide whether to keep trying
	MaxRetryDuration time.Duration
	// This function is called after MaxRetryDuration of failed retries to decide whether to keep trying
	MaxRetryDurationExceededHandler MaxRetryDurationExceededHandler

	// TLSCertHash should be nil when TLS is not enabled
	TLSCertHash []byte // util.ComputeSHA256(b.credSupport.GetClientCertificate().Certificate[0])

	sleeper sleeper

	requester *DeliveryRequester
	orderers  OrdererConnectionSource

	mutex                          sync.Mutex    // mutex protects the following fields
	stopFlag                       bool          // mark the Deliverer as stopped
	nextBlockNumber                uint64        // next block number
	lastBlockTime                  time.Time     // last block time
	fetchFailureCounter            int           // counts the number of consecutive failures to fetch a block
	fetchFailureTotalSleepDuration time.Duration // the cumulative sleep time from when fetchFailureCounter goes 0->1

	fetchSources     []*orderers.Endpoint
	fetchSourceIndex int
	fetchErrorsC     chan error

	blockReceiver     *BlockReceiver
	censorshipMonitor CensorshipDetector
}

func (d *BFTDeliverer) Initialize(channelConfig *common.Config, selfEndpoint string) {
	d.requester = NewDeliveryRequester(
		d.ChannelID,
		d.Signer,
		d.TLSCertHash,
		d.Dialer,
		d.DeliverStreamer,
	)

	osLogger := flogging.MustGetLogger("peer.orderers")
	ordererSource := d.OrderersSourceFactory.CreateConnectionSource(osLogger, selfEndpoint)
	globalAddresses, orgAddresses, err := extractAddresses(d.ChannelID, channelConfig, d.CryptoProvider)
	if err != nil {
		// The bundle was created prior to calling this function, so it should not fail when we recreate it here.
		d.Logger.Panicf("Bundle creation should not have failed: %s", err)
	}
	ordererSource.Update(globalAddresses, orgAddresses)
	d.orderers = ordererSource
}

func (d *BFTDeliverer) BlockProgress() (uint64, time.Time) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.nextBlockNumber == 0 {
		return 0, time.Time{}
	}

	return d.nextBlockNumber - 1, d.lastBlockTime
}

func (d *BFTDeliverer) MaliciousDeliverBlocks(falsifiedHeight uint64) {
	d.maliciousInitDeliverBlocks()

	defer func() {
		d.mutex.Lock()
		defer d.mutex.Unlock()

		d.Logger.Infof("Stopping to DeliverBlocks on channel `%s`, block height=%d", d.ChannelID, d.nextBlockNumber)
	}()

	timeoutConfig := TimeoutConfig{
		MinRetryInterval:       d.InitialRetryInterval,
		MaxRetryInterval:       d.MaxRetryInterval,
		BlockCensorshipTimeout: d.BlockCensorshipTimeout,
	}

	// Refresh and randomize the sources, selects a random initial source, and incurs a random iteration order.
	d.refreshSources() // 不进行源的刷新

	for {
		// Compute the backoff duration and wait before retrying.
		// The backoff duration is doubled with every failed round.
		// A failed round is when we had moved through all the endpoints without success.
		// If we get a block successfully from a source, or endpoints are refreshed, the failure count is reset.
		// 计算退避时间并等待重试, 每次失败，退避持续时间都会加倍,失败的回合是指我们遍历所有端点但都没有成功, 如果我们成功的从源获取一个块
		// 那么失败计数将被重置
		if stopLoop := d.retryBackoff(); stopLoop {
			break
		}

		// No endpoints is a non-recoverable error, as new endpoints are a result of fetching new blocks from an orderer.
		// 没有可供拉取区块的节点
		if len(d.fetchSources) == 0 {
			d.Logger.Error("Failure in DeliverBlocks, no orderer endpoints, something is critically wrong")
			break
		}

		// Start a block fetcher; a buffered channel so that the fetcher goroutine can send an error and exit w/o
		// waiting for it to be consumed. A block receiver is created within.
		d.fetchErrorsC = make(chan error, 1)

		// zhf add code 找到对应的源
		// -----------------------------------------------------
		leaderId := bft_related.ConsensusController.GetLeaderID()
		leaderAddress := config.EnvLoaderInstance.IdToAddressMapping[leaderId]
		var source *orderers.Endpoint
		fmt.Println("sources:", d.fetchSources)
		for _, fetchSource := range d.fetchSources {
			if fetchSource.Address == leaderAddress {
				source = fetchSource
				break
			} else {
				fmt.Printf("fetchSource.Address=%s,leaderAddress=%s\n", fetchSource.Address, leaderAddress)
			}
		}
		// -----------------------------------------------------

		// 查看用户选取的源
		fmt.Printf("fetch source: %v\n", source)

		// 进行区块的拉取
		go d.MaliciousFetchBlocks(source) // 会产生 fetch events

		// Create and start a censorship monitor.
		d.censorshipMonitor = d.CensorshipDetectorFactory.Create(
			d.ChannelID, d.UpdatableBlockVerifier, d.requester, d, d.fetchSources, d.fetchSourceIndex, timeoutConfig)
		go d.censorshipMonitor.Monitor() // 会产生 censorship events

		// Wait for block fetcher & censorship monitor events, or a stop signal.
		// Events which cause a retry return nil, non-recoverable errors return an error.
		if stopLoop := d.handleFetchAndCensorshipEvents(); stopLoop {
			break
		}

		d.censorshipMonitor.Stop()
	}

	// Clean up everything because we are closing
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.blockReceiver.Stop()
	if d.censorshipMonitor != nil {
		d.censorshipMonitor.Stop()
	}
}

func (d *BFTDeliverer) DeliverBlocks() {

	// original code
	// ---------------------------------------------------
	if err := d.initDeliverBlocks(); err != nil {
		d.Logger.Errorf("Failed to start DeliverBlocks: %s", err)
		return
	}
	// ---------------------------------------------------

	defer func() {
		d.mutex.Lock()
		defer d.mutex.Unlock()

		d.Logger.Infof("Stopping to DeliverBlocks on channel `%s`, block height=%d", d.ChannelID, d.nextBlockNumber)
	}()

	timeoutConfig := TimeoutConfig{
		MinRetryInterval:       d.InitialRetryInterval,
		MaxRetryInterval:       d.MaxRetryInterval,
		BlockCensorshipTimeout: d.BlockCensorshipTimeout,
	}

	// Refresh and randomize the sources, selects a random initial source, and incurs a random iteration order.
	d.refreshSources()

	for {
		// Compute the backoff duration and wait before retrying.
		// The backoff duration is doubled with every failed round.
		// A failed round is when we had moved through all the endpoints without success.
		// If we get a block successfully from a source, or endpoints are refreshed, the failure count is reset.
		// 计算退避时间并等待重试, 每次失败，退避持续时间都会加倍,失败的回合是指我们遍历所有端点但都没有成功, 如果我们成功的从源获取一个块
		// 那么失败计数将被重置
		if stopLoop := d.retryBackoff(); stopLoop {
			break
		}

		// No endpoints is a non-recoverable error, as new endpoints are a result of fetching new blocks from an orderer.
		// 没有可供拉取区块的节点
		if len(d.fetchSources) == 0 {
			d.Logger.Error("Failure in DeliverBlocks, no orderer endpoints, something is critically wrong")
			break
		}

		// Start a block fetcher; a buffered channel so that the fetcher goroutine can send an error and exit w/o
		// waiting for it to be consumed. A block receiver is created within.
		d.fetchErrorsC = make(chan error, 1)
		// 取一个源
		source := d.fetchSources[d.fetchSourceIndex]
		// 进行区块的拉取
		go d.FetchBlocks(source)

		// Create and start a censorship monitor.
		d.censorshipMonitor = d.CensorshipDetectorFactory.Create(
			d.ChannelID, d.UpdatableBlockVerifier, d.requester, d, d.fetchSources, d.fetchSourceIndex, timeoutConfig)
		go d.censorshipMonitor.Monitor()

		// Wait for block fetcher & censorship monitor events, or a stop signal.
		// Events which cause a retry return nil, non-recoverable errors return an error.
		if stopLoop := d.handleFetchAndCensorshipEvents(); stopLoop {
			break
		}

		d.censorshipMonitor.Stop()
	}

	// Clean up everything because we are closing
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.blockReceiver.Stop()
	if d.censorshipMonitor != nil {
		d.censorshipMonitor.Stop()
	}
}

// maliciousInitDeliverBlocks 恶意的进行信息初始化
func (d *BFTDeliverer) maliciousInitDeliverBlocks() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.lastBlockTime = time.Now()
	// 将当前高度存储在 nextBlockNumber 表示下一个要处理的区块号
	d.nextBlockNumber = 1

	d.Logger.Infof("Starting to DeliverBlocks on channel `%s`, block height=%d", d.ChannelID, d.nextBlockNumber)
}

// initDeliverBlocks 进行区块拉取模式的初始化
func (d *BFTDeliverer) initDeliverBlocks() (err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.lastBlockTime = time.Now()
	// 将当前高度存储在 nextBlockNumber 表示下一个要处理的区块号
	d.nextBlockNumber, err = d.Ledger.LedgerHeight()
	if err != nil {
		d.Logger.Errorf("Did not return ledger height, something is critically wrong: %s", err)
		return
	}

	d.Logger.Infof("Starting to DeliverBlocks on channel `%s`, block height=%d", d.ChannelID, d.nextBlockNumber)

	return nil
}

// retryBackoff computes the backoff duration and wait before retrying.
// The backoff duration is doubled with every failed round.
// A failed round is when we had moved through all the endpoints without success.
// If we get a block successfully from a source, or endpoints are refreshed, the failure count is reset.
func (d *BFTDeliverer) retryBackoff() (stop bool) {
	failureCounter, failureTotalSleepDuration := d.getFetchFailureStats()
	if failureCounter > 0 {
		rounds := uint(failureCounter)
		if l := len(d.fetchSources); l > 0 {
			rounds = uint(failureCounter / l)
		}

		if failureTotalSleepDuration > d.MaxRetryDuration {
			if d.MaxRetryDurationExceededHandler() {
				d.Logger.Warningf("Attempted to retry block delivery for more than MaxRetryDuration (%s), giving up", d.MaxRetryDuration)
				return true
			}
			d.Logger.Debugf("Attempted to retry block delivery for more than MaxRetryDuration (%s), but handler decided to continue retrying", d.MaxRetryDuration)
		}

		dur := backOffDuration(2.0, rounds, d.InitialRetryInterval, d.MaxRetryInterval)
		d.Logger.Warningf("Failed to fetch blocks, count=%d, round=%d, going to retry in %s", failureCounter, rounds, dur)
		d.sleeper.Sleep(dur, d.DoneC)
		d.addFetchFailureSleepDuration(dur)
	}

	return false
}

// handleFetchAndCensorshipEvents waits for events from three channels - for fetch, censorship, and done events.
// If the event is recoverable, false is returned and the main loop will switch to another block source, while
// reassigning the header receivers. If the events are non-recoverable or the stop signal, true is returned.
func (d *BFTDeliverer) handleFetchAndCensorshipEvents() (stopLoop bool) {
	d.Logger.Debug("Entry")

	select {
	case <-d.DoneC:
		d.Logger.Debug("Received the stop signal")
		return true

	case errFetch := <-d.fetchErrorsC:
		d.Logger.Debugf("Error received from fetchErrorsC channel: %s", errFetch)

		switch errFetch.(type) {
		case *ErrStopping:
			d.Logger.Debug("FetchBlocks received the stop signal")
			return true
		case *errRefreshEndpoint:
			d.Logger.Info("Refreshed endpoints, going to reassign block fetcher and censorship monitor, and reconnect to ordering service")
			d.refreshSources()
			d.resetFetchFailureCounter()
			return false
		case *ErrFatal:
			d.Logger.Errorf("Failure in FetchBlocks, something is critically wrong: %s", errFetch)
			return true
		default:
			d.Logger.Debug("FetchBlocks produced an error, going to retry")
			d.fetchSourceIndex = (d.fetchSourceIndex + 1) % len(d.fetchSources)
			d.incFetchFailureCounter()
			return false
		}

	case errMonitor := <-d.censorshipMonitor.ErrorsChannel():
		d.Logger.Debugf("Error received from censorshipMonitor.ErrorsChannel: %s", errMonitor)

		switch errMonitor.(type) {
		case *ErrStopping:
			d.Logger.Debug("CensorshipMonitor received the stop signal")
			return true
		case *ErrCensorship:
			d.Logger.Warningf("Censorship suspicion: %s; going to retry fetching blocks from another orderer", errMonitor)
			d.mutex.Lock()
			d.blockReceiver.Stop()
			d.mutex.Unlock()
			d.fetchSourceIndex = (d.fetchSourceIndex + 1) % len(d.fetchSources)
			d.incFetchFailureCounter()
			return false
		case *ErrFatal:
			d.Logger.Errorf("Failure in CensorshipMonitor, something is critically wrong: %s", errMonitor)
			return true
		default:
			d.Logger.Errorf("Unexpected error from CensorshipMonitor, something is critically wrong: %s", errMonitor)
			return true
		}
	}
}

func (d *BFTDeliverer) refreshSources() {
	// select an initial source randomly
	// 将验证器序列进行打乱
	d.fetchSources = d.orderers.ShuffledEndpoints()
	d.Logger.Infof("Refreshed endpoints: %s", d.fetchSources)
	d.fetchSourceIndex = 0
}

// Stop
func (d *BFTDeliverer) Stop() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.stopFlag {
		return
	}

	d.stopFlag = true
	close(d.DoneC)
	d.blockReceiver.Stop()
}

// MaliciousFetchBlocks zhf add code 恶意的进行区块的拉取
func (d *BFTDeliverer) MaliciousFetchBlocks(source *orderers.Endpoint) {
	d.Logger.Debugf("Trying to fetch blocks from orderer: %s", source.Address)

	for {
		select {
		case <-d.DoneC:
			d.fetchErrorsC <- &ErrStopping{Message: "stopping"}
			return
		default:
		}

		// 生成一个签名的 SeekInfo 信封，请求来自某个区块编号的区块流(区块流指的是很多的区块)。
		seekInfoEnv, err := d.requester.SeekInfoBlocksFrom(d.getNextBlockNumber())
		if err != nil {
			d.Logger.Errorf("Could not create a signed Deliver SeekInfo message, something is critically wrong: %s", err)
			d.fetchErrorsC <- &ErrFatal{Message: fmt.Sprintf("could not create a signed Deliver SeekInfo message: %s", err)}
			return
		}
		// 连接到要请求的源，请求进行发送
		deliverClient, cancel, err := d.requester.Connect(seekInfoEnv, source)
		if err != nil {
			d.Logger.Warningf("Could not connect to ordering service: %s", err)
			d.fetchErrorsC <- errors.Wrapf(err, "could not connect to ordering service, orderer-address: %s", source.Address)
			return
		}

		blockRcv := &BlockReceiver{
			channelID:              d.ChannelID,
			blockHandler:           d.BlockHandler,
			updatableBlockVerifier: d.UpdatableBlockVerifier,
			deliverClient:          deliverClient,
			cancelSendFunc:         cancel,
			recvC:                  make(chan *orderer.DeliverResponse),
			stopC:                  make(chan struct{}),
			endpoint:               source,
			logger:                 flogging.MustGetLogger("BlockReceiver").With("orderer-address", source.Address),
		}

		d.mutex.Lock()
		d.blockReceiver = blockRcv
		d.mutex.Unlock()

		// Starts a goroutine that receives blocks from the stream client and places them in the `recvC` channel
		blockRcv.Start()

		// Consume blocks from the `recvC` channel
		if errProc := blockRcv.MaliciousProcessIncoming(d.onBlockProcessingSuccess); errProc != nil {
			switch errProc.(type) {
			case *ErrStopping:
				// nothing to do
				d.Logger.Debugf("BlockReceiver stopped while processing incoming blocks: %s", errProc)
			case *errRefreshEndpoint:
				d.Logger.Infof("Endpoint refreshed while processing incoming blocks: %s", errProc)
				d.fetchErrorsC <- errProc
			default:
				d.Logger.Warningf("Failure while processing incoming blocks: %s", errProc)
				d.fetchErrorsC <- errProc
			}
			return
		}
	}
}

func (d *BFTDeliverer) FetchBlocks(source *orderers.Endpoint) {
	d.Logger.Debugf("Trying to fetch blocks from orderer: %s", source.Address)

	for {
		select {
		case <-d.DoneC:
			d.fetchErrorsC <- &ErrStopping{Message: "stopping"}
			return
		default:
		}

		// 生成一个签名的 SeekInfo 信封，请求来自某个区块编号的区块流(区块流指的是很多的区块)。
		seekInfoEnv, err := d.requester.SeekInfoBlocksFrom(d.getNextBlockNumber())
		if err != nil {
			d.Logger.Errorf("Could not create a signed Deliver SeekInfo message, something is critically wrong: %s", err)
			d.fetchErrorsC <- &ErrFatal{Message: fmt.Sprintf("could not create a signed Deliver SeekInfo message: %s", err)}
			return
		}
		// 连接到要请求的源，请求进行发送
		deliverClient, cancel, err := d.requester.Connect(seekInfoEnv, source)
		if err != nil {
			d.Logger.Warningf("Could not connect to ordering service: %s", err)
			d.fetchErrorsC <- errors.Wrapf(err, "could not connect to ordering service, orderer-address: %s", source.Address)
			return
		}

		blockRcv := &BlockReceiver{
			channelID:              d.ChannelID,
			blockHandler:           d.BlockHandler,
			updatableBlockVerifier: d.UpdatableBlockVerifier,
			deliverClient:          deliverClient,
			cancelSendFunc:         cancel,
			recvC:                  make(chan *orderer.DeliverResponse),
			stopC:                  make(chan struct{}),
			endpoint:               source,
			logger:                 flogging.MustGetLogger("BlockReceiver").With("orderer-address", source.Address),
		}

		d.mutex.Lock()
		d.blockReceiver = blockRcv
		d.mutex.Unlock()

		// Starts a goroutine that receives blocks from the stream client and places them in the `recvC` channel
		blockRcv.Start()

		// Consume blocks from the `recvC` channel
		if errProc := blockRcv.ProcessIncoming(d.onBlockProcessingSuccess); errProc != nil {
			switch errProc.(type) {
			case *ErrStopping:
				// nothing to do
				d.Logger.Debugf("BlockReceiver stopped while processing incoming blocks: %s", errProc)
			case *errRefreshEndpoint:
				d.Logger.Infof("Endpoint refreshed while processing incoming blocks: %s", errProc)
				d.fetchErrorsC <- errProc
			default:
				d.Logger.Warningf("Failure while processing incoming blocks: %s", errProc)
				d.fetchErrorsC <- errProc
			}
			return
		}
	}
}

func (d *BFTDeliverer) onBlockProcessingSuccess(blockNum uint64, channelConfig *common.Config) {
	d.Logger.Debugf("onBlockProcessingSuccess: %d, %v", blockNum, channelConfig)

	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.fetchFailureCounter = 0
	d.fetchFailureTotalSleepDuration = 0

	d.nextBlockNumber = blockNum + 1
	d.lastBlockTime = time.Now()

	fmt.Printf("zhf add code: processing next block number %d\n", d.nextBlockNumber)

	if channelConfig != nil {
		globalAddresses, orgAddresses, err := extractAddresses(d.ChannelID, channelConfig, d.CryptoProvider)
		if err != nil {
			// The bundle was created prior to calling this function, so it should not fail when we recreate it here.
			d.Logger.Panicf("Bundle creation should not have failed: %s", err)
		}
		d.Logger.Debugf("Extracted orderer addresses: global %v, orgs: %v", globalAddresses, orgAddresses)
		d.orderers.Update(globalAddresses, orgAddresses)
		d.Logger.Debugf("Updated OrdererConnectionSource")
	}

	d.Logger.Debug("onBlockProcessingSuccess: exit")
}

func (d *BFTDeliverer) resetFetchFailureCounter() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.fetchFailureCounter = 0
	d.fetchFailureTotalSleepDuration = 0
}

func (d *BFTDeliverer) getFetchFailureStats() (int, time.Duration) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.fetchFailureCounter, d.fetchFailureTotalSleepDuration
}

func (d *BFTDeliverer) incFetchFailureCounter() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.fetchFailureCounter++
}

func (d *BFTDeliverer) addFetchFailureSleepDuration(dur time.Duration) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.fetchFailureTotalSleepDuration += dur
}

func (d *BFTDeliverer) getNextBlockNumber() uint64 {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.nextBlockNumber
}

func (d *BFTDeliverer) setSleeperFunc(sleepFunc func(duration time.Duration)) {
	d.sleeper.sleep = sleepFunc
}
