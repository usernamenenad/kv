package raft

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/usernamenenad/kv/internal/transport"
)

type RaftNode struct {
	config          *RaftNodeConfig
	electionTimer   *Timer
	heartbeatTimer  *Timer
	electionTimeout time.Duration

	currentTerm   Term
	currentRole   Role
	currentLeader NodeId
	votedFor      NodeId
	votesReceived map[NodeId]struct{}

	commitLength int
	sentLength   map[NodeId]int
	ackedLength  map[NodeId]int

	log []Log

	transport transport.Transport
	rpcCh     <-chan struct{}
}

func NewRaftNode(config *RaftNodeConfig, timeout time.Duration) *RaftNode {
	return &RaftNode{
		config:          config,
		electionTimer:   NewTimer(),
		heartbeatTimer:  NewTimer(),
		electionTimeout: timeout,
		currentTerm:     0,
		currentRole:     Follower,
		votesReceived:   make(map[NodeId]struct{}),
		sentLength:      make(map[NodeId]int),
		ackedLength:     make(map[NodeId]int),
	}
}

func (r *RaftNode) resetElectionTimer() {
	jitter := time.Duration(rand.Int64N(int64(r.electionTimeout)))
	r.electionTimer.StartOrReset(r.electionTimeout + jitter)
}

// Start runs the Raft event loop until ctx is cancelled.
func (r *RaftNode) Start(ctx context.Context) {
	r.resetElectionTimer()

	for {
		select {
		case <-r.electionTimer.GetExpiryCh():
			r.StartLeaderElection(ctx)
			r.resetElectionTimer()

		case <-r.heartbeatTimer.GetExpiryCh():
			// TODO: replicate log / send heartbeats to all peers
			r.heartbeatTimer.StartOrReset(500 * time.Millisecond)

		case <-r.rpcCh:
			// RPC handlers (OnVoteRequest, OnVoteResponse, OnLogRequest) manage

		case <-ctx.Done():
			r.electionTimer.Cancel()
			r.heartbeatTimer.Cancel()
			return
		}
	}
}

func (r *RaftNode) StartLeaderElection(ctx context.Context) {
	r.currentTerm += 1
	r.currentRole = Candidate
	selfId := r.config.NodeId

	r.votedFor = selfId
	r.votesReceived = map[NodeId]struct{}{selfId: {}}

	lastTerm := Term(0)
	if len(r.log) > 0 {
		lastTerm = r.log[len(r.log)-1].term
	}

	r.transport.Broadcast(ctx, transport.Message{
		Type:   transport.VoteRequest,
		Sender: uint64(r.config.NodeId),
		Data: VoteRequestData{
			CurrentTerm: r.currentTerm,
			LogLength:   len(r.log),
			LastTerm:    lastTerm,
		},
	})
}

func (r *RaftNode) RequestBroadcast(ctx context.Context, msg transport.Message) {
	if r.currentRole == Leader {
		r.log = append(r.log, Log{
			data: msg,
			term: r.currentTerm,
		})

		r.ackedLength[r.config.NodeId] = len(r.log)
		for _, peer := range r.config.Peers {
			r.ReplicateLog(ctx, peer)
		}
	}
}

func (r *RaftNode) ReplicateLog(ctx context.Context, peerId NodeId) {
	if r.config.NodeId != r.currentLeader {
		return
	}

	prefixLen := r.sentLength[peerId]
	suffix := r.log[prefixLen:]

	prefixTerm := Term(0)
	if prefixLen > 0 {
		prefixTerm = r.log[prefixLen-1].term
	}

	r.transport.Unicast(ctx, uint64(peerId), transport.Message{
		Type:   transport.LogRequest,
		Sender: uint64(r.config.NodeId),
		Data: LogRequest{
			CurrentTerm:  r.currentTerm,
			PrefixLen:    prefixLen,
			PrefixTerm:   prefixTerm,
			CommitLength: r.commitLength,
			Suffix:       suffix,
		},
	})
}

func (r *RaftNode) OnVoteRequest(ctx context.Context, msg transport.Message) {
	data, ok := msg.Data.(VoteRequestData)
	if !ok {
		// TODO: handle faulty messages
		return
	}

	if data.CurrentTerm > r.currentTerm {
		r.currentTerm = data.CurrentTerm
		r.currentRole = Follower
		r.votedFor = NilNode
		r.heartbeatTimer.Cancel()
		r.resetElectionTimer()
	}

	lastTerm := Term(0)
	if len(r.log) > 0 {
		lastTerm = r.log[len(r.log)-1].term
	}

	logOk := (data.LastTerm > lastTerm) || (data.LastTerm == lastTerm && data.LogLength >= len(r.log))
	canVote := r.votedFor == NilNode || r.votedFor == NodeId(msg.Sender)

	if data.CurrentTerm == r.currentTerm && logOk && canVote {
		r.votedFor = NodeId(msg.Sender)
		r.resetElectionTimer()
		r.transport.Unicast(ctx, msg.Sender, transport.Message{
			Type:   transport.VoteResponse,
			Sender: uint64(r.config.NodeId),
			Data: VoteResponseData{
				VoterId:     r.config.NodeId,
				CurrentTerm: r.currentTerm,
				VoteGranted: true,
			},
		})
	} else {
		r.transport.Unicast(ctx, msg.Sender, transport.Message{
			Type:   transport.VoteResponse,
			Sender: uint64(r.config.NodeId),
			Data: VoteResponseData{
				VoterId:     r.config.NodeId,
				CurrentTerm: r.currentTerm,
				VoteGranted: false,
			},
		})
	}
}

func (r *RaftNode) OnVoteResponse(ctx context.Context, msg transport.Message) {
	data, ok := msg.Data.(VoteResponseData)
	if !ok {
		// TODO: handle faulty messages
		return
	}

	if data.CurrentTerm > r.currentTerm {
		r.currentTerm = data.CurrentTerm
		r.currentRole = Follower
		r.votedFor = NilNode
		r.heartbeatTimer.Cancel()
		r.resetElectionTimer()
		return
	}

	if r.currentRole == Candidate && data.CurrentTerm == r.currentTerm && data.VoteGranted {
		r.votesReceived[NodeId(msg.Sender)] = struct{}{}

		if len(r.votesReceived) > r.config.N/2 {
			r.currentRole = Leader
			r.currentLeader = r.config.NodeId
			r.electionTimer.Cancel()
			r.heartbeatTimer.StartOrReset(500 * time.Millisecond)

			for _, peer := range r.config.Peers {
				r.sentLength[peer] = len(r.log)
				r.ackedLength[peer] = 0
				r.ReplicateLog(ctx, peer)
			}
		}
	}
}

func (r *RaftNode) OnLogRequest(ctx context.Context, msg transport.Message) {
	data, ok := msg.Data.(LogRequest)
	if !ok {
		// TODO: handle faulty messages
		return
	}

	if data.CurrentTerm > r.currentTerm {
		r.currentTerm = data.CurrentTerm
		r.currentRole = Follower
		r.votedFor = NilNode
		r.heartbeatTimer.Cancel()
		r.resetElectionTimer()
	}

	if data.CurrentTerm == r.currentTerm {
		r.currentRole = Follower
		r.currentLeader = NodeId(msg.Sender)
		r.resetElectionTimer()
	}

	logOk := len(r.log) >= data.PrefixLen && (data.PrefixLen == 0 || r.log[data.PrefixLen-1].term == data.PrefixTerm)
	if data.CurrentTerm == r.currentTerm && logOk {
		// TODO: invoke AppendEntries method
		ack := data.PrefixLen + len(data.Suffix)
		r.transport.Unicast(ctx, msg.Sender, transport.Message{
			Type:   transport.LogResponse,
			Sender: uint64(r.config.NodeId),
			Data: LogResponse{
				CurrentTerm: r.currentTerm,
				Ack:         ack,
				Success:     true,
			},
		})
	} else {
		r.transport.Unicast(ctx, msg.Sender, transport.Message{
			Type:   transport.LogResponse,
			Sender: uint64(r.config.NodeId),
			Data: LogResponse{
				CurrentTerm: r.currentTerm,
				Ack:         0,
				Success:     false,
			},
		})
	}
}
