package sbft

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/sbft/sbftpb"
)

func (s *sbft) enqueueForSending(to peerpb.PeerID, mm []byte, certificate []byte) {
	cm := peerpb.Message{
		Content:     mm,
		Certificate: certificate,
		To:          to,
		From:        s.id,
		// TraceInfo:   trace,
	}
	s.msgs = append(s.msgs, cm)
}

func (s *sbft) marshall(m proto.Message) []byte {
	sbftMsg := pb.WrapSBFTMessage(m)
	mBytes, err := proto.Marshal(sbftMsg)
	if err != nil {
		panic("unable to marshall")
	}
	return mBytes
}

func (s *sbft) sendTo(to peerpb.PeerID, mBytes []byte, sign []byte) {
	// span := opentracing.SpanFromContext(ctx)
	// trace := make(opentracing.TextMapCarrier)
	// if err := opentracing.GlobalTracer().Inject(span.Context(), opentracing.TextMap, opentracing.TextMapWriter(trace)); err != nil {
	// 	s.logger.Panicf("error injecting trace: %w", err)
	// }

	s.enqueueForSending(to, mBytes, sign)
}

func (s *sbft) broadcast(mBytes []byte, sendToSelf bool, sign []byte) {
	// span := opentracing.SpanFromContext(ctx)
	// trace := make(opentracing.TextMapCarrier)
	// if err := opentracing.GlobalTracer().Inject(span.Context(), opentracing.TextMap, opentracing.TextMapWriter(trace)); err != nil {
	// 	s.logger.Panicf("error injecting trace: %w", err)
	// }

	for _, node := range s.nodes {
		if sendToSelf || node != s.id {
			s.enqueueForSending(node, mBytes, sign)
		}
	}
}

func (s *sbft) ClearMsgs() {
	s.msgs = nil
}
