package raft

import "slices"

type NodeId int

const NilNode NodeId = -1

type RaftNodeConfig struct {
	N      int
	NodeId NodeId
	Peers  []NodeId
}

func NewRaftNodeConfig(N int, nodeId NodeId) *RaftNodeConfig {
	return &RaftNodeConfig{
		N:      N,
		NodeId: nodeId,
		Peers:  make([]NodeId, 0),
	}
}

func (config *RaftNodeConfig) AddPeer(PeerId NodeId) {
	if slices.Contains(config.Peers, PeerId) {
		return
	}

	config.Peers = append(config.Peers, PeerId)
}
