package transport

import "context"

type Transport interface {
	Broadcast(ctx context.Context, msg Message)
	Unicast(ctx context.Context, sendTo uint64, msg Message)
}
