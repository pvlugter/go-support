package synth

import (
	"context"
	"testing"
	"time"

	"github.com/cloudstateio/go-support/cloudstate/protocol"
	"github.com/cloudstateio/go-support/tck/proto/crdt"
)

func TestCRDTPNCounter(t *testing.T) {
	s := newServer(t)
	s.newClientConn()
	defer s.teardown()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Run("PNCounter", func(t *testing.T) {
		entityId := "pncounter-1"
		p := newProxy(ctx, s)
		defer p.teardown()
		p.init(&protocol.CrdtInit{ServiceName: serviceName, EntityId: entityId})
		t.Run("incrementing a PNCounter should emit client action and create-state action", func(t *testing.T) {
			tr := tester{t}
			switch m := p.command(
				entityId, "IncrementPNCounter", &crdt.PNCounterIncrement{Key: entityId, Value: 7},
			).Message.(type) {
			case *protocol.CrdtStreamOut_Reply:
				var value crdt.PNCounterValue
				tr.toProto(m.Reply.GetClientAction().GetReply().GetPayload(), &value)
				tr.expectedInt64(value.GetValue(), 7)
				tr.expectedInt64(m.Reply.GetStateAction().GetCreate().GetPncounter().GetValue(), 7)
			default:
				tr.unexpected(m)
			}
		})
		t.Run("a second increment should emit a client action and an update-state action", func(t *testing.T) {
			tr := tester{t}
			switch m := p.command(
				entityId, "IncrementPNCounter", &crdt.PNCounterIncrement{Key: entityId, Value: 7},
			).Message.(type) {
			case *protocol.CrdtStreamOut_Reply:
				var value crdt.PNCounterValue
				tr.toProto(m.Reply.GetClientAction().GetReply().GetPayload(), &value)
				tr.expectedInt64(value.GetValue(), 14)
				tr.expectedInt64(m.Reply.GetStateAction().GetUpdate().GetPncounter().GetChange(), 7)
			default:
				tr.unexpected(m)
			}
		})
		t.Run("a decrement should emit a client action and an update state action", func(t *testing.T) {
			tr := tester{t}
			switch m := p.command(
				entityId, "DecrementPNCounter", &crdt.PNCounterDecrement{Key: entityId, Value: 28},
			).Message.(type) {
			case *protocol.CrdtStreamOut_Reply:
				var value crdt.PNCounterValue
				tr.toProto(m.Reply.GetClientAction().GetReply().GetPayload(), &value)
				tr.expectedInt64(value.GetValue(), -14)
				tr.expectedInt64(m.Reply.GetStateAction().GetUpdate().GetPncounter().GetChange(), -28)
			default:
				tr.unexpected(m)
			}
		})
		t.Run("the counter should apply new state and return its value", func(t *testing.T) {
			tr := tester{t}
			p.state(&protocol.PNCounterState{Value: 49})
			switch m := p.command(
				entityId, "GetPNCounter", &crdt.Get{Key: entityId},
			).Message.(type) {
			case *protocol.CrdtStreamOut_Reply:
				var value crdt.PNCounterValue
				tr.toProto(m.Reply.GetClientAction().GetReply().GetPayload(), &value)
				tr.expectedInt64(value.GetValue(), 49)
				tr.expectedNil(m.Reply.GetClientAction().GetFailure())
			default:
				tr.unexpected(m)
			}
		})
		t.Run("the counter should apply a delta and return its value", func(t *testing.T) {
			tr := tester{t}
			p.delta(&protocol.PNCounterDelta{Change: -56})
			switch m := p.command(
				entityId, "GetPNCounter", &crdt.Get{Key: entityId},
			).Message.(type) {
			case *protocol.CrdtStreamOut_Reply:
				var value crdt.PNCounterValue
				tr.toProto(m.Reply.GetClientAction().GetReply().GetPayload(), &value)
				tr.expectedInt64(value.GetValue(), -7)
			default:
				tr.unexpected(m)
			}
		})
	})
}