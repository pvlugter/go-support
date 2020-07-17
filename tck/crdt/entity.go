package crdt

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cloudstateio/go-support/cloudstate/crdt"
	"github.com/cloudstateio/go-support/cloudstate/encoding"
	tc "github.com/cloudstateio/go-support/tck/proto/crdt"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
)

type SyntheticCRDTs struct {
	id        crdt.EntityId
	gCounter  *crdt.GCounter
	pnCounter *crdt.PNCounter
	gSet      *crdt.GSet
	orSet     *crdt.ORSet
	vote      *crdt.Vote
}

func NewEntity(id crdt.EntityId) *SyntheticCRDTs {
	return &SyntheticCRDTs{id: id}
}

func (s *SyntheticCRDTs) Set(_ *crdt.Context, c crdt.CRDT) {
	switch v := c.(type) {
	case *crdt.GCounter:
		s.gCounter = v
	case *crdt.PNCounter:
		s.pnCounter = v
	case *crdt.GSet:
		s.gSet = v
	case *crdt.ORSet:
		s.orSet = v
	case *crdt.Vote:
		s.vote = v
	}
}

func (s *SyntheticCRDTs) Default(c *crdt.Context) crdt.CRDT {
	if strings.HasPrefix(c.EntityId.String(), "gcounter-") {
		return crdt.NewGCounter()
	}
	if strings.HasPrefix(c.EntityId.String(), "pncounter-") {
		return crdt.NewPNCounter()
	}
	if strings.HasPrefix(c.EntityId.String(), "gset-") {
		return crdt.NewGSet()
	}
	if strings.HasPrefix(c.EntityId.String(), "orset-") {
		return crdt.NewORSet()
	}
	if strings.HasPrefix(c.EntityId.String(), "vote-") {
		return crdt.NewVote()
	}
	c.Fail(errors.New("unknown entity type"))
	return nil
}

func (s *SyntheticCRDTs) HandleCommand(_ *crdt.CommandContext, name string, cmd proto.Message) (*any.Any, error) {
	defer checkToFail(cmd)
	fmt.Printf("got: %v, %+v\n", name, cmd)
	switch name {
	case "IncrementGCounter":
		switch c := cmd.(type) {
		case *tc.GCounterIncrement:
			s.gCounter.Increment(c.GetValue())
			pb := &tc.GCounterValue{Value: s.gCounter.Value()}
			fmt.Printf("ret: %+v\n", pb)
			return encoding.MarshalAny(pb)
		}
	case "GetGCounter":
		return encoding.MarshalAny(&tc.GCounterValue{Value: s.gCounter.Value()})
	case "IncrementPNCounter":
		switch c := cmd.(type) {
		case *tc.PNCounterIncrement:
			s.pnCounter.Increment(c.Value)
			return encoding.MarshalAny(&tc.PNCounterValue{Value: s.pnCounter.Value()})
		}
	case "DecrementPNCounter":
		switch c := cmd.(type) {
		case *tc.PNCounterDecrement:
			s.pnCounter.Decrement(c.Value)
			return encoding.MarshalAny(&tc.PNCounterValue{Value: s.pnCounter.Value()})
		}
	case "GetPNCounter":
		return encoding.MarshalAny(&tc.PNCounterValue{Value: s.pnCounter.Value()})
	case "AddGSet":
		switch c := cmd.(type) {
		case *tc.GSetAdd:
			anySupportAdd(s.gSet, c.Value)
			return encoding.MarshalAny(&tc.GSetValue{Values: s.gSet.Value()})
		}
	case "GetGSet":
		return encoding.MarshalAny(&tc.GSetValue{Values: s.gSet.Value()})
	case "GetGSetAnySupport":
		v := tc.GSetValueAnySupport{Values: make([]*tc.AnySupportType, 0, len(s.gSet.Value()))}
		for _, a := range s.gSet.Value() {
			v.Values = append(v.Values, &tc.AnySupportType{
				Value: asAnySupportType(a).Value},
			)
		}
		return encoding.MarshalAny(&v)
	case "GetGSetSize":
		return encoding.MarshalAny(&tc.GSetSize{Value: int64(s.gSet.Size())})

	case "AddORSet":
		switch c := cmd.(type) {
		case *tc.ORSetAdd:
			anySupportAdd(s.orSet, c.Value)
			return encoding.MarshalAny(&tc.ORSetValue{Values: s.orSet.Value()})
		}
	case "RemoveORSet":
		anySupportRemove(s.orSet, cmd.(*tc.ORSetRemove).Value)
		return encoding.MarshalAny(&tc.ORSetValue{Values: s.orSet.Value()})
	case "GetORSet":
		return encoding.MarshalAny(&tc.ORSetValue{Values: s.orSet.Value()})
	case "GetORSetSize":
		return encoding.MarshalAny(&tc.ORSetSize{Value: int64(s.orSet.Size())})
	}
	return nil, errors.New("unhandled command")
}

func asAnySupportType(x *any.Any) *tc.AnySupportType {
	switch x.TypeUrl {
	case encoding.PrimitiveTypeURLPrefixBool:
		return &tc.AnySupportType{
			Value: &tc.AnySupportType_BoolValue{BoolValue: encoding.DecodeBool(x)},
		}
	case encoding.PrimitiveTypeURLPrefixBytes:
		return &tc.AnySupportType{
			Value: &tc.AnySupportType_BytesValue{BytesValue: encoding.DecodeBytes(x)},
		}
	case encoding.PrimitiveTypeURLPrefixFloat:
		return &tc.AnySupportType{
			Value: &tc.AnySupportType_FloatValue{FloatValue: encoding.DecodeFloat32(x)},
		}
	case encoding.PrimitiveTypeURLPrefixDouble:
		return &tc.AnySupportType{
			Value: &tc.AnySupportType_DoubleValue{DoubleValue: encoding.DecodeFloat64(x)},
		}
	case encoding.PrimitiveTypeURLPrefixInt32:
		return &tc.AnySupportType{
			Value: &tc.AnySupportType_Int32Value{Int32Value: encoding.DecodeInt32(x)},
		}
	case encoding.PrimitiveTypeURLPrefixInt64:
		return &tc.AnySupportType{
			Value: &tc.AnySupportType_Int64Value{Int64Value: encoding.DecodeInt64(x)},
		}
	case encoding.PrimitiveTypeURLPrefixString:
		return &tc.AnySupportType{
			Value: &tc.AnySupportType_StringValue{StringValue: encoding.DecodeString(x)},
		}
	}
	panic(fmt.Sprintf("no mapping found for TypeUrl: %v", x.TypeUrl)) // we're allowed to panic here :)
}

type anySupportAdder interface {
	Add(x *any.Any)
}

type anySupportRemover interface {
	Remove(x *any.Any)
}

func anySupportRemove(r anySupportRemover, t *tc.AnySupportType) {
	switch v := t.Value.(type) {
	case *tc.AnySupportType_AnyValue:
		r.Remove(v.AnyValue)
	case *tc.AnySupportType_StringValue:
		r.Remove(encoding.String(v.StringValue))
	case *tc.AnySupportType_BytesValue:
		r.Remove(encoding.Bytes(v.BytesValue))
	case *tc.AnySupportType_BoolValue:
		r.Remove(encoding.Bool(v.BoolValue))
	case *tc.AnySupportType_DoubleValue:
		r.Remove(encoding.Float64(v.DoubleValue))
	case *tc.AnySupportType_FloatValue:
		r.Remove(encoding.Float32(v.FloatValue))
	case *tc.AnySupportType_Int32Value:
		r.Remove(encoding.Int32(v.Int32Value))
	case *tc.AnySupportType_Int64Value:
		r.Remove(encoding.Int64(v.Int64Value))
	}
}

func anySupportAdd(a anySupportAdder, t *tc.AnySupportType) {
	switch v := t.Value.(type) {
	case *tc.AnySupportType_AnyValue:
		a.Add(v.AnyValue)
	case *tc.AnySupportType_StringValue:
		a.Add(encoding.String(v.StringValue))
	case *tc.AnySupportType_BytesValue:
		a.Add(encoding.Bytes(v.BytesValue))
	case *tc.AnySupportType_BoolValue:
		a.Add(encoding.Bool(v.BoolValue))
	case *tc.AnySupportType_DoubleValue:
		a.Add(encoding.Float64(v.DoubleValue))
	case *tc.AnySupportType_FloatValue:
		a.Add(encoding.Float32(v.FloatValue))
	case *tc.AnySupportType_Int32Value:
		a.Add(encoding.Int32(v.Int32Value))
	case *tc.AnySupportType_Int64Value:
		a.Add(encoding.Int64(v.Int64Value))
	}
}

func checkToFail(c interface{}) {
	if i, ok := c.(interface{ GetFail() bool }); ok && i.GetFail() {
		panic("forced crash")
	}
}
