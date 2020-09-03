package synth

import (
	"context"
	"testing"
	"time"

	"github.com/cloudstateio/go-support/cloudstate/protocol"
	"github.com/cloudstateio/go-support/tck/proto/crdt"
	"github.com/golang/protobuf/ptypes/empty"
)

func TestCRDTFlag(t *testing.T) {
	s := newServer(t)
	s.newClientConn()
	defer s.teardown()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Run("Flag", func(t *testing.T) {
		entityId := "flag-1"
		command := "ProcessFlag"
		p := newProxy(ctx, s)
		defer p.teardown()

		p.init(&protocol.CrdtInit{ServiceName: serviceName, EntityId: entityId})
		t.Run("Get emits client action", func(t *testing.T) {
			tr := tester{t}
			switch m := p.command(entityId, command,
				flagRequest(&crdt.Get{}),
			).Message.(type) {
			case *protocol.CrdtStreamOut_Reply:
				tr.expectedNil(m.Reply.GetSideEffects())
				tr.expectedNil(m.Reply.GetClientAction().GetFailure())
				tr.expectedNil(m.Reply.GetStateAction().GetCreate())
				// action reply
				var f crdt.FlagValue
				tr.toProto(m.Reply.GetClientAction().GetReply().GetPayload(), &f)
				tr.expectedFalse(f.GetValue())
			default:
				tr.unexpected(m)
			}
		})
		t.Run("FlagEnable emits client action and create state action", func(t *testing.T) {
			tr := tester{t}
			switch m := p.command(entityId, command,
				flagRequest(&crdt.FlagEnable{}),
			).Message.(type) {
			case *protocol.CrdtStreamOut_Reply:
				tr.expectedNil(m.Reply.GetSideEffects())
				tr.expectedNil(m.Reply.GetClientAction().GetFailure())
				// action reply
				var f crdt.FlagValue
				tr.toProto(m.Reply.GetClientAction().GetReply().GetPayload(), &f)
				tr.expectedTrue(f.GetValue())
				tr.expectedNotNil(m.Reply.GetStateAction().GetCreate())
				tr.expectedTrue(m.Reply.GetStateAction().GetCreate().GetFlag().GetValue())
			default:
				tr.unexpected(m)
			}
		})
		t.Run("Delete emits client action and delete state action", func(t *testing.T) {
			tr := tester{t}
			switch m := p.command(entityId, command,
				flagRequest(&crdt.Delete{}),
			).Message.(type) {
			case *protocol.CrdtStreamOut_Reply:
				tr.expectedNil(m.Reply.GetSideEffects())
				tr.expectedNil(m.Reply.GetClientAction().GetFailure())
				e := &empty.Empty{}
				tr.toProto(m.Reply.GetClientAction().GetReply().GetPayload(), e)
				// action reply
				tr.expectedNil(m.Reply.GetStateAction().GetCreate())
				tr.expectedNil(m.Reply.GetStateAction().GetUpdate())
				tr.expectedNil(m.Reply.GetStateAction().GetDelete())
			default:
				tr.unexpected(m)
			}
		})
		t.Run("FlagState should reflect state", func(t *testing.T) {
			tr := tester{t}
			p := newProxy(ctx, s)
			defer p.teardown()

			entityId = "flag-2"
			p.init(&protocol.CrdtInit{ServiceName: serviceName, EntityId: entityId})
			p.state(&protocol.FlagState{Value: true})
			switch m := p.command(entityId, command,
				flagRequest(&crdt.Get{}),
			).Message.(type) {
			case *protocol.CrdtStreamOut_Reply:
				tr.expectedNil(m.Reply.GetSideEffects())
				tr.expectedNil(m.Reply.GetClientAction().GetFailure())
				tr.expectedNil(m.Reply.GetStateAction().GetCreate())
				// action reply
				var f crdt.FlagValue
				tr.toProto(m.Reply.GetClientAction().GetReply().GetPayload(), &f)
				tr.expectedTrue(f.GetValue())
			default:
				tr.unexpected(m)
			}
		})
		t.Run("FlagDelta should reflect state", func(t *testing.T) {
			tr := tester{t}
			p := newProxy(ctx, s)
			defer p.teardown()

			entityId = "flag-3"
			p.init(&protocol.CrdtInit{ServiceName: serviceName, EntityId: entityId})
			p.delta(&protocol.FlagDelta{Value: true})
			switch m := p.command(entityId, command,
				flagRequest(&crdt.Get{}),
			).Message.(type) {
			case *protocol.CrdtStreamOut_Reply:
				tr.expectedNil(m.Reply.GetSideEffects())
				tr.expectedNil(m.Reply.GetClientAction().GetFailure())
				tr.expectedNil(m.Reply.GetStateAction().GetCreate())
				// action reply
				var f crdt.FlagValue
				tr.toProto(m.Reply.GetClientAction().GetReply().GetPayload(), &f)
				tr.expectedTrue(f.GetValue())
			default:
				tr.unexpected(m)
			}
		})
	})
}
