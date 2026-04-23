package raft

type Role uint8
type Term uint64

const (
	Follower Role = iota
	Candidate
	Leader
)

type VoteRequestData struct {
	Term      Term
	LastTerm  Term
	LogLength int
}

type VoteResponseData struct {
	VoterId     NodeId
	Term        Term
	VoteGranted bool
}

type LogRequest struct {
	Term         Term
	PrefixLen    int
	PrefixTerm   Term
	CommitLength int
	Suffix       []Log
}

type LogResponse struct {
	Term    Term
	Ack     int
	Success bool
}
