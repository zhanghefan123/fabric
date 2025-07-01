# 1. 修改的文件

- fabric-3.0.0/internal/peer/node/start.go 添加了启动运行 frr 的过程
- fabric-3.0.0/cmd/orderer/main.go 添加了启动运行 frr 的过程
- 上述两个启动项共同使用 fabric-3.0.0/zeusnet/starter.go 之中的启动程序

# 2. smartBFT 的启动入口

- orderer/consensus/smartbft/chain.go 之中的 Start 方法

# 3. 当一个orderer 断开连接的时候会触发这个函数

```
// OnRequestTimeout is called when request-timeout expires and forwards the request to leader.
// Called by the request-pool timeout goroutine. Upon return, the leader-forward timeout is started.
func (c *Controller) OnRequestTimeout(request []byte, info types.RequestInfo) {
	iAm, leaderID := c.iAmTheLeader()
	if iAm {
		c.Logger.Infof("Request %s timeout expired, this node is the leader, nothing to do", info)
		return
	}

	c.Logger.Infof("Request %s timeout expired, forwarding request to leader: %d", info, leaderID)
	c.Comm.SendTransaction(leaderID, request)
}
```

# 4. 弄懂一下  RequestComplainTimeout 的具体含义


# 5. 启动函数
- smartbft/pkg/consensus/consensus.go 之中的 startComponents 函数

```
// startComponents smartBFT 之中的不同的组件的启动函数
func (c *Consensus) startComponents(view, seq, dec uint64, configSync bool) {
	// If we delivered to the application proposal with sequence i,
	// then we are expecting to be proposed a proposal with sequence i+1.
	c.collector.Start()
	c.viewChanger.Start(view)
	if configSync {
		c.controller.Start(view, seq+1, dec, c.Config.SyncOnStart)
	} else {
		c.controller.Start(view, seq+1, dec, false)
	}
}
```

# 6. 可以在 log 之中搜索

```
got message from
```

从而发现节点收到了什么消息而执行了什么流程


# 7. 处理消息的模块 smartbft/internal/bft/controller.go

```go
// ProcessMessages dispatches the incoming message to the required component
func (c *Controller) ProcessMessages(sender uint64, m *protos.Message) {
	c.Logger.Debugf("%d got message from %d: %s", c.ID, sender, MsgToString(m))
	switch m.GetContent().(type) {
	case *protos.Message_PrePrepare, *protos.Message_Prepare, *protos.Message_Commit:
		c.currViewLock.RLock()
		view := c.currView
		c.currViewLock.RUnlock()
		view.HandleMessage(sender, m)
		c.ViewChanger.HandleViewMessage(sender, m)
		if sender == c.leaderID() {
			c.LeaderMonitor.InjectArtificialHeartbeat(sender, c.convertViewMessageToHeartbeat(m))
		}
	case *protos.Message_ViewChange, *protos.Message_ViewData, *protos.Message_NewView:
		c.ViewChanger.HandleMessage(sender, m)
	case *protos.Message_HeartBeat, *protos.Message_HeartBeatResponse:
		c.LeaderMonitor.ProcessMsg(sender, m)
	case *protos.Message_StateTransferRequest:
		c.respondToStateTransferRequest(sender)
	case *protos.Message_StateTransferResponse:
		c.Collector.HandleMessage(sender, m)
	default:
		c.Logger.Warnf("Unexpected message type, ignoring")
	}
}
```


smartbft/internal/bft/view.go

```go
func (v *View) processMsg(sender uint64, m *protos.Message) {
	if v.Stopped() {
		return
	}
	// Ensure view number is equal to our view
	msgViewNum := viewNumber(m)
	msgProposalSeq := proposalSequence(m)

	if msgViewNum != v.Number {
		v.Logger.Warnf("%d got message %v from %d of view %d, expected view %d", v.SelfID, m, sender, msgViewNum, v.Number)
		if sender != v.LeaderID {
			v.discoverIfSyncNeeded(sender, m)
			return
		}
		v.FailureDetector.Complain(v.Number, false)
		// Else, we got a message with a wrong view from the leader.
		if msgViewNum > v.Number {
			v.Sync.Sync()
		}
		v.stop()
		return
	}

	if msgProposalSeq == v.ProposalSequence-1 && v.ProposalSequence > 0 {
		v.handlePrevSeqMessage(msgProposalSeq, sender, m)
		return
	}

	v.Logger.Debugf("%d got message %s from %d with seq %d", v.SelfID, MsgToString(m), sender, msgProposalSeq)
	// This message is either for this proposal or the next one (we might be behind the rest)
	if msgProposalSeq != v.ProposalSequence && msgProposalSeq != v.ProposalSequence+1 {
		v.Logger.Warnf("%d got message from %d with sequence %d but our sequence is %d", v.SelfID, sender, msgProposalSeq, v.ProposalSequence)
		v.discoverIfSyncNeeded(sender, m)
		return
	}

	msgForNextProposal := msgProposalSeq == v.ProposalSequence+1

	if pp := m.GetPrePrepare(); pp != nil {
		v.processPrePrepare(pp, m, msgForNextProposal, sender)
		return
	}

	// Else, it's a prepare or a commit.
	// Ignore votes from ourselves.
	if sender == v.SelfID {
		return
	}

	if prp := m.GetPrepare(); prp != nil {
		if msgForNextProposal {
			v.nextPrepares.registerVote(sender, m)
		} else {
			v.prepares.registerVote(sender, m)
		}
		return
	}

	if cmt := m.GetCommit(); cmt != nil {
		if msgForNextProposal {
			v.nextCommits.registerVote(sender, m)
		} else {
			v.commits.registerVote(sender, m)
		}
		return
	}
}
```

pkg/gateway/submit.go 用来进行交易的提交，从 grpc_client 发送给 grpc_server, 接着 grpc_server 进行 submit

```
// Submit will send the signed transaction to the ordering service. The response indicates whether the transaction was
// successfully received by the orderer. This does not imply successful commit of the transaction, only that is has
// been delivered to the orderer.
func (gs *Server) Submit(ctx context.Context, request *gp.SubmitRequest) (*gp.SubmitResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "a submit request is required")
	}
	txn := request.GetPreparedTransaction()
	if txn == nil {
		return nil, status.Error(codes.InvalidArgument, "a prepared transaction is required")
	}
	if len(txn.Signature) == 0 {
		return nil, status.Error(codes.InvalidArgument, "prepared transaction must be signed")
	}
	orderers, clusterSize, err := gs.registry.orderers(request.ChannelId)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%s", err)
	}

	if len(orderers) == 0 {
		return nil, status.Errorf(codes.Unavailable, "no orderer nodes available")
	}

	logger := logger.With("txID", request.TransactionId)
	config := gs.getChannelConfig(request.ChannelId)
	oc, ok := config.OrdererConfig()
	if !ok {
		return nil, status.Errorf(codes.NotFound, "failed to create block deliverer for channel `%s`, missing OrdererConfig", request.ChannelId)
	}
	if oc.ConsensusType() == "BFT" {
		return gs.submitBFT(ctx, orderers, txn, clusterSize, logger)
	} else {
		return gs.submitNonBFT(ctx, orderers, txn, logger)
	}
}
```


这个函数进行了输出最后的关闭流程
```
// Close kicks off the shutdown process of the transport. This should be called
// only once on a transport. Once it is called, the transport should not be
// accessed anymore.
func (t *http2Client) Close(err error) {
	t.conn.SetWriteDeadline(time.Now().Add(time.Second * 10))
	t.mu.Lock()
	// Make sure we only close once.
	if t.state == closing {
		t.mu.Unlock()
		return
	}
	if t.logger.V(logLevel) {
		t.logger.Infof("Closing: %v", err)
	}
	// Call t.onClose ASAP to prevent the client from attempting to create new
	// streams.
	if t.state != draining {
		t.onClose(GoAwayInvalid)
	}
	t.state = closing
	streams := t.activeStreams
	t.activeStreams = nil
	if t.kpDormant {
		// If the keepalive goroutine is blocked on this condition variable, we
		// should unblock it so that the goroutine eventually exits.
		t.kpDormancyCond.Signal()
	}
	t.mu.Unlock()

	// Per HTTP/2 spec, a GOAWAY frame must be sent before closing the
	// connection. See https://httpwg.org/specs/rfc7540.html#GOAWAY. It
	// also waits for loopyWriter to be closed with a timer to avoid the
	// long blocking in case the connection is blackholed, i.e. TCP is
	// just stuck.
	t.controlBuf.put(&goAway{code: http2.ErrCodeNo, debugData: []byte("client transport shutdown"), closeConn: err})
	timer := time.NewTimer(goAwayLoopyWriterTimeout)
	defer timer.Stop()
	select {
	case <-t.writerDone: // success
	case <-timer.C:
		t.logger.Infof("Failed to write a GOAWAY frame as part of connection close after %s. Giving up and closing the transport.", goAwayLoopyWriterTimeout)
	}
	t.cancel()
	t.conn.Close()
	channelz.RemoveEntry(t.channelz.ID)
	// Append info about previous goaways if there were any, since this may be important
	// for understanding the root cause for this connection to be closed.
	_, goAwayDebugMessage := t.GetGoAwayReason()

	var st *status.Status
	if len(goAwayDebugMessage) > 0 {
		st = status.Newf(codes.Unavailable, "closing transport due to: %v, received prior goaway: %v", err, goAwayDebugMessage)
		err = st.Err()
	} else {
		st = status.New(codes.Unavailable, err.Error())
	}

	// Notify all active streams.
	for _, s := range streams {
		t.closeStream(s, err, false, http2.ErrCodeNo, st, nil, false)
	}
	for _, sh := range t.statsHandlers {
		connEnd := &stats.ConnEnd{
			Client: true,
		}
		sh.HandleConn(t.ctx, connEnd)
	}
}
```

直接down掉接口可能对端收不到正常的 TCP 结束报文, 从而导致可能还是能够发送正常的报文。

这个函数负责进行向 peer 进行发送

```shell
func (h *Handler) deliverBlocks(ctx context.Context, srv *Server, envelope *cb.Envelope) (status cb.Status, err error) {
	addr := util.ExtractRemoteAddress(ctx)
	payload, chdr, shdr, err := h.parseEnvelope(ctx, envelope)
	if err != nil {
		logger.Warningf("error parsing envelope from %s: %s", addr, err)
		return cb.Status_BAD_REQUEST, nil
	}

	chain := h.ChainManager.GetChain(chdr.ChannelId)
	if chain == nil {
		// Note, we log this at DEBUG because SDKs will poll waiting for channels to be created
		// So we would expect our log to be somewhat flooded with these
		logger.Debugf("Rejecting deliver for %s because channel %s not found", addr, chdr.ChannelId)
		return cb.Status_NOT_FOUND, nil
	}

	labels := []string{
		"channel", chdr.ChannelId,
		"filtered", strconv.FormatBool(isFiltered(srv)),
		"data_type", srv.DataType(),
	}
	h.Metrics.RequestsReceived.With(labels...).Add(1)
	defer func() {
		labels := append(labels, "success", strconv.FormatBool(status == cb.Status_SUCCESS))
		h.Metrics.RequestsCompleted.With(labels...).Add(1)
	}()

	seekInfo := &ab.SeekInfo{}
	if err = proto.Unmarshal(payload.Data, seekInfo); err != nil {
		logger.Warningf("[channel: %s] Received a signed deliver request from %s with malformed seekInfo payload: %s", chdr.ChannelId, addr, err)
		return cb.Status_BAD_REQUEST, nil
	}

	erroredChan := chain.Errored()
	if seekInfo.ErrorResponse == ab.SeekInfo_BEST_EFFORT {
		// In a 'best effort' delivery of blocks, we should ignore consenter errors
		// and continue to deliver blocks according to the client's request.
		erroredChan = nil
	}
	select {
	case <-erroredChan:
		logger.Warningf("[channel: %s] Rejecting deliver request for %s because of consenter error", chdr.ChannelId, addr)
		return cb.Status_SERVICE_UNAVAILABLE, nil
	default:
	}

	accessControl, err := NewSessionAC(chain, envelope, srv.PolicyChecker, chdr.ChannelId, h.ExpirationCheckFunc)
	if err != nil {
		logger.Warningf("[channel: %s] failed to create access control object due to %s", chdr.ChannelId, err)
		return cb.Status_BAD_REQUEST, nil
	}

	if err := accessControl.Evaluate(); err != nil {
		logger.Warningf("[channel: %s] Client %s is not authorized: %s", chdr.ChannelId, addr, err)
		return cb.Status_FORBIDDEN, nil
	}

	if seekInfo.Start == nil || seekInfo.Stop == nil {
		logger.Warningf("[channel: %s] Received seekInfo message from %s with missing start or stop %v, %v", chdr.ChannelId, addr, seekInfo.Start, seekInfo.Stop)
		return cb.Status_BAD_REQUEST, nil
	}

	logger.Debugf("[channel: %s] Received seekInfo (%p) %v from %s", chdr.ChannelId, seekInfo, seekInfo, addr)

	cursor, number := chain.Reader().Iterator(seekInfo.Start)
	defer cursor.Close()
	var stopNum uint64
	switch stop := seekInfo.Stop.Type.(type) {
	case *ab.SeekPosition_Oldest:
		stopNum = number
	case *ab.SeekPosition_Newest:
		// when seeking only the newest block (i.e. starting
		// and stopping at newest), don't reevaluate the ledger
		// height as this can lead to multiple blocks being
		// sent when only one is expected
		if proto.Equal(seekInfo.Start, seekInfo.Stop) {
			stopNum = number
			break
		}
		stopNum = chain.Reader().Height() - 1
	case *ab.SeekPosition_Specified:
		stopNum = stop.Specified.Number
		if stopNum < number {
			logger.Warningf("[channel: %s] Received invalid seekInfo message from %s: start number %d greater than stop number %d", chdr.ChannelId, addr, number, stopNum)
			return cb.Status_BAD_REQUEST, nil
		}
	}

	for {
		if seekInfo.Behavior == ab.SeekInfo_FAIL_IF_NOT_READY {
			if number > chain.Reader().Height()-1 {
				logger.Warningf("[channel: %s] Block %d not found, block number greater than chain length bounds", chdr.ChannelId, number)
				return cb.Status_NOT_FOUND, nil
			}
		}

		var block *cb.Block
		var status cb.Status

		iterCh := make(chan struct{})
		go func() {
			block, status = cursor.Next()
			close(iterCh)
		}()

		select {
		case <-ctx.Done():
			logger.Debugf("Context canceled, aborting wait for next block")
			return cb.Status_INTERNAL_SERVER_ERROR, errors.Wrapf(ctx.Err(), "context finished before block retrieved")
		case <-erroredChan:
			// TODO, today, the only user of the errorChan is the orderer consensus implementations.  If the peer ever reports
			// this error, we will need to update this error message, possibly finding a way to signal what error text to return.
			logger.Warningf("Aborting deliver for request because the backing consensus implementation indicates an error")
			return cb.Status_SERVICE_UNAVAILABLE, nil
		case <-iterCh:
			// Iterator has set the block and status vars
		}

		if status != cb.Status_SUCCESS {
			logger.Warningf("[channel: %s] Error reading from channel, cause was: %v", chdr.ChannelId, status)
			return status, nil
		}

		// increment block number to support FAIL_IF_NOT_READY deliver behavior
		number++

		if err := accessControl.Evaluate(); err != nil {
			logger.Warningf("[channel: %s] Client authorization revoked for deliver request from %s: %s", chdr.ChannelId, addr, err)
			return cb.Status_FORBIDDEN, nil
		}

		logger.Debugf("[channel: %s] Delivering block [%d] for (%p) for %s", chdr.ChannelId, block.Header.Number, seekInfo, addr)

		// Data blocks carry nil data for block attestations.
		// Never mutate the block received from the iterator as it is from a cache.
		block2send := block
		if seekInfo.ContentType == ab.SeekInfo_HEADER_WITH_SIG && !protoutil.IsConfigBlock(block) {
			block2send = &cb.Block{
				Header:   block.Header,
				Metadata: block.Metadata,
			}
		}

		signedData := &protoutil.SignedData{Data: envelope.Payload, Identity: shdr.Creator, Signature: envelope.Signature}
		// zhf add code: SendBlockResponse 如果不能及时的进行发送的话
		if err := srv.SendBlockResponse(block2send, chdr.ChannelId, chain, signedData); err != nil {
			logger.Warningf("[channel: %s] Error sending to %s: %s", chdr.ChannelId, addr, err)
			return cb.Status_INTERNAL_SERVER_ERROR, err
		}

		h.Metrics.BlocksSent.With(labels...).Add(1)

		if stopNum == block.Header.Number {
			break
		}
	}

	logger.Debugf("[channel: %s] Done delivering to %s for (%p)", chdr.ChannelId, addr, seekInfo)

	return cb.Status_SUCCESS, nil
}

```

下面的代码将排序的一系列 block 发送给 client
```go
// Deliver sends a stream of blocks to a client after ordering
func (s *server) Deliver(srv ab.AtomicBroadcast_DeliverServer) error {
	logger.Debugf("Starting new Deliver handler")
	defer func() {
		if r := recover(); r != nil {
			logger.Criticalf("Deliver client triggered panic: %s\n%s", r, debug.Stack())
		}
		logger.Debugf("Closing Deliver stream")
	}()

	policyChecker := func(env *cb.Envelope, channelID string) error {
		chain := s.GetChain(channelID)
		if chain == nil {
			return errors.Errorf("channel %s not found", channelID)
		}
		// In maintenance mode, we typically require the signature of /Channel/Orderer/Readers.
		// This will block Deliver requests from peers (which normally satisfy /Channel/Readers).
		sf := msgprocessor.NewSigFilter(policies.ChannelReaders, policies.ChannelOrdererReaders, chain)
		return sf.Apply(env)
	}
	deliverServer := &deliver.Server{
		PolicyChecker: deliver.PolicyCheckerFunc(policyChecker),
		Receiver: &deliverMsgTracer{
			Receiver: srv,
			msgTracer: msgTracer{
				debug:    s.debug,
				function: "Deliver",
			},
		},
		ResponseSender: &responseSender{
			AtomicBroadcast_DeliverServer: srv,
		},
	}
	return s.dh.Handle(srv.Context(), deliverServer)
}

```