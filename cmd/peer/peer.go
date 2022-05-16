package main

import (
	_ "net/http/pprof"

	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/protocols/chainhotstuff"
	"github.com/ibalajiarun/go-consensus/protocols/destiny"
	"github.com/ibalajiarun/go-consensus/protocols/dispel"
	"github.com/ibalajiarun/go-consensus/protocols/dqpbft"
	"github.com/ibalajiarun/go-consensus/protocols/dqsbftslow"
	"github.com/ibalajiarun/go-consensus/protocols/duobft"
	"github.com/ibalajiarun/go-consensus/protocols/hybster"
	"github.com/ibalajiarun/go-consensus/protocols/hybsterx"
	"github.com/ibalajiarun/go-consensus/protocols/linhybster"
	"github.com/ibalajiarun/go-consensus/protocols/minbft"
	"github.com/ibalajiarun/go-consensus/protocols/mirbft"
	"github.com/ibalajiarun/go-consensus/protocols/multichainduobft"
	"github.com/ibalajiarun/go-consensus/protocols/multichainduobft/chainduobft"
	"github.com/ibalajiarun/go-consensus/protocols/pbft"
	"github.com/ibalajiarun/go-consensus/protocols/prime"
	"github.com/ibalajiarun/go-consensus/protocols/rcc"
	"github.com/ibalajiarun/go-consensus/protocols/sbft"
	"github.com/ibalajiarun/go-consensus/protocols/sbftslow"
	"github.com/ibalajiarun/go-consensus/protocols/sbftx"
	"github.com/ibalajiarun/go-consensus/transport"
)

func newPeer(cfg *peer.LocalConfig) *peer.Peer {
	t := transport.NewGRPCTransport()

	var p peer.Protocol
	switch cfg.Algorithm {
	case peerpb.Algorithm_PBFT:
		p = pbft.NewPBFT(cfg)
	case peerpb.Algorithm_SBFT:
		p = sbft.NewSBFT(cfg)
	case peerpb.Algorithm_Hybster:
		p = hybster.NewHybster(cfg)
	case peerpb.Algorithm_Destiny:
		p = destiny.NewDestiny(cfg)
	case peerpb.Algorithm_LinHybster:
		p = linhybster.NewLinHybster(cfg)
	case peerpb.Algorithm_SBFTSlow:
		p = sbftslow.NewSBFTSlow(cfg)
	case peerpb.Algorithm_Prime:
		p = prime.NewPrime(cfg)
	case peerpb.Algorithm_MinBFT:
		p = minbft.NewMinBFT(cfg)
	case peerpb.Algorithm_DuoBFT:
		p = duobft.NewDuoBFT(cfg)
	case peerpb.Algorithm_Hybsterx:
		p = hybsterx.NewHybsterx(cfg)
	case peerpb.Algorithm_ChainHotstuff:
		p = chainhotstuff.NewChainHotstuff(cfg)
	case peerpb.Algorithm_RCC:
		p = rcc.NewRCC(cfg)
	case peerpb.Algorithm_MirBFT:
		p = mirbft.NewMirBFT(cfg)
	case peerpb.Algorithm_Dispel:
		p = dispel.NewDispel(cfg)
	case peerpb.Algorithm_SBFTx:
		p = sbftx.NewSBFTx(cfg)
	case peerpb.Algorithm_DQPBFT:
		p = dqpbft.NewDQPBFT(cfg)
	case peerpb.Algorithm_DQSBFTSlow:
		p = dqsbftslow.NewDQSBFTSlow(cfg)
	case peerpb.Algorithm_ChainDuoBFT:
		p = chainduobft.NewChainDuoBFT(cfg)
	case peerpb.Algorithm_MultiChainDuoBFT:
		p = multichainduobft.NewMultiChainDuoBFT(cfg)
	case peerpb.Algorithm_MultiChainDuoBFTRCC:
		cfg.MultiChainDuoBFT.RCCMode = true
		p = multichainduobft.NewMultiChainDuoBFT(cfg)
	default:
		cfg.Logger.Panicf("Unknown protocol specified %v", cfg.Algorithm)
	}

	return peer.New(cfg, t, p)
}
