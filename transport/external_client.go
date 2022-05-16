package transport

import (
	"context"
	"time"

	transpb "github.com/ibalajiarun/go-consensus/transport/transportpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
)

type ExternalClient struct {
	transpb.CommandServiceClient
	*grpc.ClientConn
}

func NewExternalClient(ctx context.Context, addr string) (*ExternalClient, error) {
	conn, err := grpc.DialContext(ctx, addr, grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  1.0 * time.Second,
				Multiplier: 1.5,
				Jitter:     0.2,
				MaxDelay:   10 * time.Second,
			},
		}),
		grpc.WithInitialWindowSize(1<<20),
	)

	if err != nil {
		return nil, err
	}
	client := transpb.NewCommandServiceClient(conn)
	return &ExternalClient{client, conn}, nil
}
