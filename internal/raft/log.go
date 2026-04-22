package raft

type Log struct {
	data any
	term Term
}
