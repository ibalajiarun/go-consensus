package main

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/transport/transportpb"
	"github.com/ibalajiarun/go-consensus/utils/helper"
	"github.com/ibalajiarun/go-consensus/utils/signer"
)

func TestPayloadSize(t *testing.T) {
	signer := signer.NewSigner()

	batchSize := 200
	payloadSize := 500

	idPrefix := uint64(10) << 48
	rid := idPrefix | uint64(1)
	key := fmt.Sprintf("key-%d", idPrefix)
	keyBytes := []byte(key)

	value := make([]byte, payloadSize*int(batchSize))
	rand.Read(value)

	ops := make([]commandpb.Operation, batchSize)
	for i := range ops {
		ops[i] = commandpb.Operation{
			Type: &commandpb.Operation_KVOp{
				KVOp: &commandpb.KVOp{
					Key:   keyBytes,
					Value: value[i*payloadSize : (i+1)*payloadSize],
					Read:  false,
				},
			},
		}
	}
	fmt.Println(ops[0].Size())
	cmd := &commandpb.Command{
		Timestamp: rid,
		Ops:       ops,
		Meta:      nil,
	}
	fmt.Println(cmd.Size())
	cmdBytes, sig := helper.MarshallAndSign(cmd, signer)
	cp := &transportpb.ClientPacket{
		Message:   cmdBytes,
		Signature: sig,
	}
	fmt.Println(cp.Size())

	t.Fail()
}
