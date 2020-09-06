//
// Copyright 2020 Lightbend Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"github.com/golang/protobuf/ptypes/empty"
)

type SyntheticCRDTs struct {
	id          crdt.EntityId
	gCounter    *crdt.GCounter
	pnCounter   *crdt.PNCounter
	gSet        *crdt.GSet
	orSet       *crdt.ORSet
	flag        *crdt.Flag
	lwwRegister *crdt.LWWRegister
	vote        *crdt.Vote
	orMap       *crdt.ORMap
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
	case *crdt.Flag:
		s.flag = v
	case *crdt.LWWRegister:
		s.lwwRegister = v
	case *crdt.Vote:
		s.vote = v
	case *crdt.ORMap:
		s.orMap = v
	}
}

func (s *SyntheticCRDTs) Default(c *crdt.Context) (crdt.CRDT, error) {
	switch strings.Split(c.EntityId.String(), "-")[0] {
	case "gcounter":
		return crdt.NewGCounter(), nil
	case "pncounter":
		return crdt.NewPNCounter(), nil
	case "gset":
		return crdt.NewGSet(), nil
	case "orset":
		return crdt.NewORSet(), nil
	case "flag":
		return crdt.NewFlag(), nil
	case "lwwregister":
		return crdt.NewLWWRegister(nil), nil
	case "vote":
		return crdt.NewVote(), nil
	case "ormap":
		return crdt.NewORMap(), nil
	default:
		return nil, errors.New("unknown entity type")
	}
}

func (s *SyntheticCRDTs) HandleCommand(cc *crdt.CommandContext, _ string, cmd proto.Message) (*any.Any, error) {
	switch c := cmd.(type) {
	case *tc.GCounterRequest:
		for _, as := range c.GetActions() {
			switch a := as.Action.(type) {
			case *tc.GCounterRequestAction_Increment:
				s.gCounter.Increment(a.Increment.GetValue())
				v := &tc.GCounterValue{Value: s.gCounter.Value()}
				if with := a.Increment.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.GCounterResponse{Value: v})
			case *tc.GCounterRequestAction_Get:
				if with := a.Get.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.GCounterResponse{Value: &tc.GCounterValue{Value: s.gCounter.Value()}})
			case *tc.GCounterRequestAction_Delete:
				cc.Delete()
				if with := a.Delete.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.GCounterResponse{})
			}
		}
	case *tc.PNCounterRequest:
		for _, as := range c.GetActions() {
			switch a := as.Action.(type) {
			case *tc.PNCounterRequestAction_Increment:
				s.pnCounter.Increment(a.Increment.Value)
				if with := a.Increment.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.PNCounterResponse{Value: &tc.PNCounterValue{Value: s.pnCounter.Value()}})
			case *tc.PNCounterRequestAction_Decrement:
				s.pnCounter.Decrement(a.Decrement.Value)
				if with := a.Decrement.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.PNCounterResponse{Value: &tc.PNCounterValue{Value: s.pnCounter.Value()}})
			case *tc.PNCounterRequestAction_Get:
				if with := a.Get.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.PNCounterResponse{Value: &tc.PNCounterValue{Value: s.pnCounter.Value()}})
			case *tc.PNCounterRequestAction_Delete:
				cc.Delete()
				if with := a.Delete.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.PNCounterResponse{})
			}
		}
	case *tc.GSetRequest:
		for _, as := range c.GetActions() {
			switch a := as.Action.(type) {
			case *tc.GSetRequestAction_Get:
				v := make([]*tc.AnySupportType, 0, len(s.gSet.Value()))
				for _, a := range s.gSet.Value() {
					v = append(v, asAnySupportType(a))
				}
				if with := a.Get.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.GSetResponse{Value: &tc.GSetValue{Values: v}})
			case *tc.GSetRequestAction_Delete:
				cc.Delete()
				if with := a.Delete.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.GSetResponse{})
			case *tc.GSetRequestAction_Add:
				anySupportAdd(s.gSet, a.Add.Value)
				v := make([]*tc.AnySupportType, 0, len(s.gSet.Value()))
				for _, a := range s.gSet.Value() {
					if strings.HasPrefix(a.TypeUrl, encoding.JSONTypeURLPrefix) {
						v = append(v, &tc.AnySupportType{
							Value: &tc.AnySupportType_AnyValue{AnyValue: a},
						})
						continue
					}
					v = append(v, asAnySupportType(a))
				}
				if with := a.Add.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.GSetResponse{Value: &tc.GSetValue{Values: v}})
			}
		}
	case *tc.ORSetRequest:
		for _, as := range c.GetActions() {
			switch a := as.Action.(type) {
			case *tc.ORSetRequestAction_Get:
				if with := a.Get.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.ORSetResponse{Value: &tc.ORSetValue{Values: s.orSet.Value()}})
			case *tc.ORSetRequestAction_Delete:
				cc.Delete()
				if with := a.Delete.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.ORSetResponse{})
			case *tc.ORSetRequestAction_Add:
				anySupportAdd(s.orSet, a.Add.Value)
				if with := a.Add.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.ORSetResponse{Value: &tc.ORSetValue{Values: s.orSet.Value()}})
			case *tc.ORSetRequestAction_Remove:
				anySupportRemove(s.orSet, a.Remove.Value)
				if with := a.Remove.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.ORSetResponse{Value: &tc.ORSetValue{Values: s.orSet.Value()}})
			}
		}
	case *tc.FlagRequest:
		for _, as := range c.GetActions() {
			switch a := as.Action.(type) {
			case *tc.FlagRequestAction_Get:
				if with := a.Get.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.FlagResponse{Value: &tc.FlagValue{Value: s.flag.Value()}})
			case *tc.FlagRequestAction_Delete:
				cc.Delete()
				if with := a.Delete.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.FlagResponse{})
			case *tc.FlagRequestAction_Enable:
				if with := a.Enable.FailWith; with != "" {
					return nil, errors.New(with)
				}
				s.flag.Enable()
				return encoding.MarshalAny(&tc.FlagResponse{Value: &tc.FlagValue{Value: s.flag.Value()}})
			}
		}
	case *tc.LWWRegisterRequest:
		for _, as := range c.GetActions() {
			switch a := as.Action.(type) {
			case *tc.LWWRegisterRequestAction_Get:
				if with := a.Get.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.LWWRegisterResponse{Value: &tc.LWWRegisterValue{Value: s.lwwRegister.Value()}})
			case *tc.LWWRegisterRequestAction_Delete:
				cc.Delete()
				if with := a.Delete.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.FlagResponse{})
			case *tc.LWWRegisterRequestAction_Set:
				if with := a.Set.FailWith; with != "" {
					return nil, errors.New(with)
				}
				anySupportAdd(&anySupportAdderSetter{s.lwwRegister}, a.Set.GetValue())
				return encoding.MarshalAny(&tc.LWWRegisterResponse{Value: &tc.LWWRegisterValue{Value: s.lwwRegister.Value()}})
			case *tc.LWWRegisterRequestAction_SetWithClock:
				if with := a.SetWithClock.FailWith; with != "" {
					return nil, errors.New(with)
				}
				anySupportSetClock(
					s.lwwRegister,
					a.SetWithClock.GetValue(),
					crdt.Clock(uint64(a.SetWithClock.GetClock().Number())),
					a.SetWithClock.CustomClockValue,
				)
				return encoding.MarshalAny(&tc.LWWRegisterResponse{Value: &tc.LWWRegisterValue{Value: s.lwwRegister.Value()}})
			}
		}
	case *tc.VoteRequest:
		for _, as := range c.GetActions() {
			switch a := as.Action.(type) {
			case *tc.VoteRequestAction_Get:
				if with := a.Get.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.VoteResponse{
					SelfVote: s.vote.SelfVote(),
					Voters:   s.vote.Voters(),
					VotesFor: s.vote.VotesFor(),
				})
			case *tc.VoteRequestAction_Delete:
				cc.Delete()
				if with := a.Delete.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&empty.Empty{})
			case *tc.VoteRequestAction_Vote:
				s.vote.Vote(a.Vote.GetValue())
				if with := a.Vote.FailWith; with != "" {
					return nil, errors.New(with)
				}
				return encoding.MarshalAny(&tc.VoteResponse{
					SelfVote: s.vote.SelfVote(),
					Voters:   s.vote.Voters(),
					VotesFor: s.vote.VotesFor(),
				})
			}
		}
	case *tc.ORMapRequest:
		for _, as := range c.GetActions() {
			switch a := as.Action.(type) {
			case *tc.ORMapRequestAction_Get:
				if with := a.Get.FailWith; with != "" {
					return nil, errors.New(with)
				}
				// entries := make([]*protocol.ORMapEntry, 0)
				for _, v := range s.orMap.Values() {
					v.GetState()
				}
				return encoding.MarshalAny(&tc.ORMapResponse{})
			case *tc.ORMapRequestAction_Delete:
				if with := a.Delete.FailWith; with != "" {
					return nil, errors.New(with)
				}
			case *tc.ORMapRequestAction_SetKey:
				// we reuse this entities implementation to
				// handle requests for a certain CRDT of this
				// ORMaps value.
				cm := &SyntheticCRDTs{}
				switch t := a.SetKey.GetRequest().(type) {
				case *tc.ORMapSet_GCounterRequest:
					ctx := &crdt.CommandContext{Context: &crdt.Context{EntityId: "gcounter"}}
					if !s.orMap.HasKey(a.SetKey.EntryKey) {
						counter, _ := cm.Default(ctx.Context)
						cm.Set(ctx.Context, counter)
						s.orMap.SetGCounter(a.SetKey.EntryKey, cm.gCounter)
					} else {
						counter, _ := s.orMap.GCounter(a.SetKey.EntryKey)
						cm.Set(ctx.Context, counter)
					}
					_, _ = cm.HandleCommand(ctx, "", t.GCounterRequest)
					return encoding.MarshalAny(&tc.ORMapResponse{
						Entries: &tc.ORMapEntries{
							Values: append(make([]*tc.ORMapEntry, 0), &tc.ORMapEntry{
								EntryKey: a.SetKey.EntryKey,
								Value:    encoding.Struct(cm.gCounter.State()),
							}),
						},
					})
				case *tc.ORMapSet_PnCounterRequest:
				}
				if with := a.SetKey.FailWith; with != "" {
					return nil, errors.New(with)
				}
			case *tc.ORMapRequestAction_DeleteKey:
				if with := a.DeleteKey.FailWith; with != "" {
					return nil, errors.New(with)
				}
			}
		}
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

type anySupportSetter interface {
	Set(x *any.Any)
}

type anySupportAdderSetter struct {
	anySupportSetter
}

func (s *anySupportAdderSetter) Add(x *any.Any) {
	s.Set(x)
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

func anySupportSetClock(r *crdt.LWWRegister, t *tc.AnySupportType, clock crdt.Clock, customValue int64) {
	switch v := t.Value.(type) {
	case *tc.AnySupportType_AnyValue:
		r.SetWithClock(v.AnyValue, clock, customValue)
	case *tc.AnySupportType_StringValue:
		r.SetWithClock(encoding.String(v.StringValue), clock, customValue)
	case *tc.AnySupportType_BytesValue:
		r.SetWithClock(encoding.Bytes(v.BytesValue), clock, customValue)
	case *tc.AnySupportType_BoolValue:
		r.SetWithClock(encoding.Bool(v.BoolValue), clock, customValue)
	case *tc.AnySupportType_DoubleValue:
		r.SetWithClock(encoding.Float64(v.DoubleValue), clock, customValue)
	case *tc.AnySupportType_FloatValue:
		r.SetWithClock(encoding.Float32(v.FloatValue), clock, customValue)
	case *tc.AnySupportType_Int32Value:
		r.SetWithClock(encoding.Int32(v.Int32Value), clock, customValue)
	case *tc.AnySupportType_Int64Value:
		r.SetWithClock(encoding.Int64(v.Int64Value), clock, customValue)
	}
}
