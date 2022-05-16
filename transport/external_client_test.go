package transport

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestExternalClient(t *testing.T) {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("error listenting %v", err)
	}
	srv := grpc.NewServer()
	go func() {
		time.Sleep(4 * time.Second)
		srv.Serve(lis)
	}()
	start := time.Now()
	cli, err := NewExternalClient(context.TODO(), lis.Addr().String())
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("error %v", err)
	}
	cli.Close()
	if elapsed <= 5*time.Second || elapsed >= 6*time.Second {
		t.Fatalf("client only took %v", elapsed)
	}
}
