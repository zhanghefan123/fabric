# 1. 由于是阻塞的原因, 所以需要进行下列的操作

```go
// BroadcastConsensus broadcasts the message and informs the heartbeat monitor if necessary
func (c *Controller) BroadcastConsensus(m *protos.Message) {
	// 遍历所有的节点, 除了自己进行共识消息的发送
	for _, node := range c.NodesList {
		// Do not send to yourself
		if c.ID == node {
			continue
		}
		// 发送的 m 是心跳消息
		go func() {
			c.Comm.SendConsensus(node, m)
		}()
	}
	// 如果消息是 PrePrepare, Prepare, Commit, 心跳消息显然不是
	if m.GetPrePrepare() != nil || m.GetPrepare() != nil || m.GetCommit() != nil {
		if leader, _ := c.iAmTheLeader(); leader {
			c.LeaderMonitor.HeartbeatWasSent()
		}
	}
}
```


```go
func (a *csAttempt) getTransport() error {
	cs := a.cs

	var err error

	// zhf add code
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)             // zhf add code
	a.t, a.pickResult, err = cs.cc.getTransport(ctx, cs.callInfo.failFast, cs.callHdr.Method) // zhf add code
	// a.t, a.pickResult, err = cs.cc.getTransport(a.ctx, cs.callInfo.failFast, cs.callHdr.Method) // modified code
	if err != nil {
		if de, ok := err.(dropError); ok {
			err = de.error
			a.drop = true
		}
		cancel() // zhf add code
		return err
	}
	if a.trInfo != nil {
		a.trInfo.firstLine.SetRemoteAddr(a.t.RemoteAddr())
	}
	cancel() // zhf add code
	return nil
}
```