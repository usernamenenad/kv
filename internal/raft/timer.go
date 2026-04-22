package raft

import (
	"time"
)

type Timer struct {
	timer *time.Timer
}

func NewTimer() *Timer {
	return &Timer{}
}

func (t *Timer) StartOrReset(duration time.Duration) {
	if t.timer == nil {
		t.timer = time.NewTimer(duration)
		return
	}

	if !t.timer.Stop() {
		select {
		case <-t.timer.C:
		default:
		}
	}

	t.timer.Reset(duration)
}

func (t *Timer) Cancel() {
	if t.timer == nil {
		return
	}
	if !t.timer.Stop() {
		select {
		case <-t.timer.C:
		default:
		}
	}
	t.timer = nil
}

func (t *Timer) GetExpiryCh() <-chan time.Time {
	if t.timer == nil {
		return nil
	}
	return t.timer.C
}
