// Copyright IBM Corp. All Rights Reserved.
//
// SPDX-License-Identifier: Apache-2.0
//

package api

import (
	bft "github.com/hyperledger-labs/SmartBFT/pkg/types"
	protos "github.com/hyperledger-labs/SmartBFT/smartbftprotos"
)

// Application delivers the consented proposal and corresponding signatures. 应用提交经过同意的提案以及相应的签名
type Application interface {
	// Deliver delivers the given proposal and signatures.
	// After the call returns we assume that this proposal is stored in persistent memory.
	// It returns whether this proposal was a reconfiguration and the current config.
	Deliver(proposal bft.Proposal, signature []bft.Signature) bft.Reconfig
}

// Comm enables the communications between the nodes. 节点之间通信的接口
type Comm interface {
	// SendConsensus sends the consensus protocol related message m to the node with id targetID.
	SendConsensus(targetID uint64, m *protos.Message)
	// SendTransaction sends the given client's request to the node with id targetID.
	SendTransaction(targetID uint64, request []byte)
	// Nodes returns a set of ids of participating nodes.
	// In case you need to change or keep this slice, create a copy.
	Nodes() []uint64
}

// Assembler creates proposals. 创建提案的接口
type Assembler interface {
	// AssembleProposal creates a proposal which includes
	// the given requests (when permitting) and metadata.
	AssembleProposal(metadata []byte, requests [][]byte) bft.Proposal
}

// WriteAheadLog is write ahead log. 预写日志
type WriteAheadLog interface {
	// Append appends a data item to the end of the WAL
	// and indicate whether this entry is a truncation point.
	// Append 将会将一个数据项添加到预写日志的末尾, 并且指示此条目是否为断点
	Append(entry []byte, truncateTo bool) error
}

// Signer signs on the given data. 对于给定数据进行签名
type Signer interface {
	// Sign signs on the given data and returns the signature.
	// 对给定数据签名
	Sign([]byte) []byte
	// SignProposal signs on the given proposal and returns a composite Signature.
	// 对于给定的提案进行签名
	SignProposal(proposal bft.Proposal, auxiliaryInput []byte) *bft.Signature
}

// Verifier validates data and verifies signatures.
// 验证数据以及签名
type Verifier interface {
	// VerifyProposal verifies the given proposal and returns the included requests' info.
	// 验证提案并返回包含的请求信息
	VerifyProposal(proposal bft.Proposal) ([]bft.RequestInfo, error)
	// VerifyRequest verifies the given request and returns its info.
	// 验证请求并返回他的信息
	VerifyRequest(val []byte) (bft.RequestInfo, error)
	// VerifyConsenterSig verifies the signature for the given proposal.
	// It returns the auxiliary data in the signature.
	// 验证给定提案的签名
	VerifyConsenterSig(signature bft.Signature, prop bft.Proposal) ([]byte, error)
	// VerifySignature verifies the signature.
	// 验证签名
	VerifySignature(signature bft.Signature) error
	// VerificationSequence returns the current verification sequence.
	// 返回当前验证序列
	VerificationSequence() uint64
	// RequestsFromProposal returns from the given proposal the included requests' info
	// 返回 proposal 之中包含的 request
	RequestsFromProposal(proposal bft.Proposal) []bft.RequestInfo
	// AuxiliaryData extracts the auxiliary data from a signature's message
	// 从签名信息中提取辅助数据
	AuxiliaryData([]byte) []byte
}

// MembershipNotifier notifies if there was a membership change in the last proposal.
// 提示最后一个提案中是否有成员资格变更
type MembershipNotifier interface {
	// MembershipChange returns true if there was a membership change in the last proposal.
	MembershipChange() bool
}

// RequestInspector extracts info (i.e. request id and client id) from a given request.
// 从 request 之中 提取 ID
type RequestInspector interface {
	// RequestID returns info about the given request.
	RequestID(req []byte) bft.RequestInfo
}

// Synchronizer reaches the cluster nodes and fetches blocks in order to sync the replica's state.
// 从其他节点进行区块的同步
type Synchronizer interface {
	// Sync blocks indefinitely until the replica's state is synchronized to the latest decision,
	// and returns it with info about reconfiguration.
	Sync() bft.SyncResponse
	StartMaliciousSync() error
	StopMaliciousSync() error
	GetBlockHeight() int
}

// Logger defines the contract for logging.
type Logger interface {
	Debugf(template string, args ...interface{})
	Infof(template string, args ...interface{})
	Errorf(template string, args ...interface{})
	Warnf(template string, args ...interface{})
	Panicf(template string, args ...interface{})
}
