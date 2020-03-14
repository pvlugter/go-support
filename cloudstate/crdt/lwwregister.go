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
	"github.com/cloudstateio/go-support/cloudstate/protocol"
	"github.com/golang/protobuf/ptypes/any"
)

type LWWRegister struct {
	value            *any.Any
	clock            Clock
	customClockValue int64
	delta            lwwRegisterDelta
}

type lwwRegisterDelta struct {
	value            *any.Any
	clock            Clock
	customClockValue int64
}

func NewLWWRegister(x *any.Any) *LWWRegister {
	return NewLWWRegisterWithClock(x, Default, 0)
}

func NewLWWRegisterWithClock(x *any.Any, c Clock, customClockValue int64) *LWWRegister {
	return &LWWRegister{
		value:            x,
		clock:            c,
		customClockValue: customClockValue,
		delta:            lwwRegisterDelta{},
	}
}

func (r LWWRegister) Value() *any.Any {
	return r.value
}

func (r *LWWRegister) Set(x *any.Any) {
	r.SetWithClock(x, Default, 0)
}

// The custom clock value to use if the clock selected is a custom clock.
// This is ignored if the clock is not a custom clock
func (r *LWWRegister) SetWithClock(x *any.Any, c Clock, customClockValue int64) {
	r.value = x
	r.clock = c
	r.customClockValue = customClockValue
	r.delta = lwwRegisterDelta{
		value:            x,
		clock:            c,
		customClockValue: customClockValue,
	}
}

func (r *LWWRegister) Delta() *protocol.LWWRegisterDelta {
	return &protocol.LWWRegisterDelta{
		Value:            r.delta.value,
		Clock:            r.delta.clock.toCrdtClock(),
		CustomClockValue: r.delta.customClockValue,
	}
}

func (r *LWWRegister) HasDelta() bool {
	return r.delta.value != nil
}

func (r *LWWRegister) ResetDelta() {
	r.delta = lwwRegisterDelta{}
	r.clock = Default
	r.customClockValue = 0
}

func (r *LWWRegister) ApplyDelta(d *protocol.LWWRegisterDelta) {
	r.value = d.Value
}

func (r LWWRegister) State() *protocol.LWWRegisterState {
	return &protocol.LWWRegisterState{
		Value:            r.value,
		Clock:            r.clock.toCrdtClock(),
		CustomClockValue: r.customClockValue,
	}
}

func (r *LWWRegister) ApplyState(d *protocol.LWWRegisterState) {
	r.value = d.Value
	r.clock = fromCrdtClock(d.Clock)
	r.customClockValue = d.CustomClockValue
}
