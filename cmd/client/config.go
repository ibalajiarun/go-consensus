package main

import (
	"context"
	"os"
	"strings"

	masterpb "github.com/ibalajiarun/go-consensus/cmd/master/masterpb"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"google.golang.org/grpc"
)

func getConfigFromMaster(maddr string) (*masterpb.ClientResponse, []int, error) {
	conn, err := grpc.Dial(maddr, grpc.WithInsecure())
	if err != nil {
		return nil, nil, err
	}
	cli := masterpb.NewServiceDiscoveryClient(conn)

	hostname := os.Getenv("HOSTNAME")
	region := hostname
	pieces := strings.Split(hostname, "-")
	if len(pieces) == 4 {
		region = pieces[1]
	}

	nid := &peerpb.BasicPeerInfo{
		PodName:     os.Getenv("PODNAME"),
		HostMachine: hostname,
		PodIP:       os.Getenv("PODIP"),
		Region:      region,
	}
	result, err := cli.GetServiceInfo(context.Background(), nid)
	if err != nil {
		return nil, nil, err
	}

	var localIDs []int
	for _, s := range result.Config.Nodes {
		if s.Region == region {
			localIDs = append(localIDs, int(s.PeerID))
		}
	}

	return result, localIDs, nil
}
