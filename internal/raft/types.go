package raft

type Role uint8
type Term uint64

const (
	Follower Role = iota
	Candidate
	Leader
)

type VoteRequestData struct {
	CurrentTerm Term
	LastTerm    Term
	LogLength   int
}

type VoteResponseData struct {
	VoterId     NodeId
	CurrentTerm Term
	VoteGranted bool
}

type LogRequest struct {
	CurrentTerm  Term
	PrefixLen    int
	PrefixTerm   Term
	CommitLength int
	Suffix       []Log
}

type LogResponse struct {
	CurrentTerm Term
	Ack         int
	Success     bool
}
