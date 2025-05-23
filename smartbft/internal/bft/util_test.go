// Copyright IBM Corp. All Rights Reserved.
//
// SPDX-License-Identifier: Apache-2.0
//

package bft

import (
	"fmt"
	"testing"

	"github.com/hyperledger-labs/SmartBFT/pkg/api"
	"github.com/hyperledger-labs/SmartBFT/pkg/metrics/disabled"
	"github.com/hyperledger-labs/SmartBFT/pkg/types"
	protos "github.com/hyperledger-labs/SmartBFT/smartbftprotos"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestBlacklist(t *testing.T) {
	for _, tst := range []struct {
		preparesFrom       map[uint64]*protos.PreparesFrom
		prevMD             *protos.ViewMetadata
		decisionsPerLeader uint64
		currView           uint64
		currentLeader      uint64
		leaderRotation     bool
		expected           []uint64
		nodes              []uint64
		name               string
	}{
		{
			name:               "Node added to blacklist not first proposal and dont blacklist current leader",
			expected:           []uint64{2, 3},
			decisionsPerLeader: 1,
			nodes:              []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			leaderRotation:     true,
			currView:           3,
			currentLeader:      4,
			prevMD: &protos.ViewMetadata{
				ViewId:          0,
				LatestSequence:  1,
				DecisionsInView: 0,
				BlackList:       nil,
			},
			preparesFrom: map[uint64]*protos.PreparesFrom{},
		},
		{
			name:               "Node added to blacklist first proposal",
			expected:           []uint64{getLeaderID(1, 4, []uint64{1, 2, 3, 4}, true, 0, 1, []uint64{1, 2})},
			decisionsPerLeader: 1,
			nodes:              []uint64{1, 2, 3, 4},
			leaderRotation:     true,
			currView:           2,
			prevMD: &protos.ViewMetadata{
				ViewId:          1,
				LatestSequence:  0,
				DecisionsInView: 1,
				BlackList:       []uint64{1, 2}, // Blacklist is already over capacity
			},
			preparesFrom: map[uint64]*protos.PreparesFrom{},
		},
		{
			name:               "Node removed from blacklist due to attestations",
			expected:           nil,
			decisionsPerLeader: 1,
			nodes:              []uint64{1, 2, 3, 4},
			leaderRotation:     true,
			currView:           1,
			prevMD: &protos.ViewMetadata{
				ViewId:          1,
				LatestSequence:  1,
				DecisionsInView: 1,
				BlackList:       []uint64{3},
			},
			preparesFrom: map[uint64]*protos.PreparesFrom{
				2: {Ids: []uint64{3}},
				3: {Ids: []uint64{3}},
			},
		},
		{
			name:               "Node removed from blacklist due to being removed from nodes",
			expected:           nil,
			decisionsPerLeader: 1,
			leaderRotation:     true,
			nodes:              []uint64{1, 2, 3, 4},
			currView:           1,
			prevMD: &protos.ViewMetadata{
				ViewId:          1,
				LatestSequence:  1,
				DecisionsInView: 1,
				BlackList:       []uint64{5},
			},
			preparesFrom: map[uint64]*protos.PreparesFrom{},
		},
	} {
		t.Run(tst.name, func(t *testing.T) {
			logConfig := zap.NewDevelopmentConfig()
			logger, _ := logConfig.Build()

			bl := blacklist{
				currentLeader:      tst.currentLeader,
				n:                  uint64(len(tst.nodes)),
				nodes:              tst.nodes,
				logger:             logger.Sugar(),
				metricsBlacklist:   api.NewMetricsBlacklist(&disabled.Provider{}),
				f:                  (len(tst.nodes) - 1) / 3,
				currView:           tst.currView,
				preparesFrom:       tst.preparesFrom,
				prevMD:             tst.prevMD,
				leaderRotation:     tst.leaderRotation,
				decisionsPerLeader: tst.decisionsPerLeader,
			}

			assert.Equal(t, tst.expected, bl.computeUpdate())
		})
	}
}

func TestInFlightProposalLatest(t *testing.T) {
	prop := types.Proposal{
		VerificationSequence: 1,
		Metadata:             []byte{1},
		Payload:              []byte{2},
		Header:               []byte{3},
	}

	ifp := &InFlightData{}
	assert.Nil(t, ifp.InFlightProposal())

	ifp.StoreProposal(prop)
	assert.Equal(t, prop, *ifp.InFlightProposal())
}

func TestQuorum(t *testing.T) {
	// Ensure that quorum size is as expected.

	type quorum struct {
		N uint64
		F int
		Q int
	}

	quorums := []quorum{
		{4, 1, 3},
		{5, 1, 4},
		{6, 1, 4},
		{7, 2, 5},
		{8, 2, 6},
		{9, 2, 6},
		{10, 3, 7},
		{11, 3, 8},
		{12, 3, 8},
	}

	for _, testCase := range quorums {
		t.Run(fmt.Sprintf("%d nodes", testCase.N), func(t *testing.T) {
			Q, F := computeQuorum(testCase.N)
			assert.Equal(t, testCase.Q, Q)
			assert.Equal(t, testCase.F, F)
		})
	}
}

func TestGetLeaderId(t *testing.T) {
	nodes := []uint64{1, 2, 3, 4}
	view := uint64(0)

	decisionsPerLeader := uint64(1)
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(3), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(4), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(3), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(4), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 2
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(3), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(3), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(4), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(4), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 3
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(3), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(3), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(3), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 4
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(3), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 5
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 6
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 7
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 8
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(2), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 9
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 10
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(1), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	nodes = []uint64{11, 12, 13, 14, 15}
	decisionsPerLeader = 1
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 2
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 3
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 4
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 5
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	view = 1
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 6
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(12), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	view = 2
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 7
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(13), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	view = 3
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 8
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(14), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	view = 4
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 9
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(15), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	view = 5
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))

	decisionsPerLeader = 10
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 0, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 1, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 2, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 3, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 4, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 5, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 6, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 7, decisionsPerLeader, nil))
	assert.Equal(t, uint64(11), getLeaderID(view, uint64(len(nodes)), nodes, true, 8, decisionsPerLeader, nil))
}
