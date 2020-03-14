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
	"hash/maphash"

	"github.com/cloudstateio/go-support/cloudstate/protocol"
	"github.com/golang/protobuf/ptypes/any"
)

type ORSet struct {
	value   map[uint64]*any.Any
	added   map[uint64]*any.Any
	removed map[uint64]*any.Any
	cleared bool
	anyHasher
}

func NewORSet() *ORSet {
	return &ORSet{
		value:     make(map[uint64]*any.Any),
		added:     make(map[uint64]*any.Any),
		removed:   make(map[uint64]*any.Any),
		cleared:   false,
		anyHasher: anyHasher(maphash.MakeSeed()),
	}
}

func (s *ORSet) Size() int {
	return len(s.value)
}

func (s *ORSet) Add(a *any.Any) {
	h := s.hashAny(a)
	if _, exists := s.value[h]; !exists {
		if _, has := s.removed[h]; has {
			delete(s.removed, h)
		} else {
			s.added[h] = a
		}
		s.value[h] = a
	}
}

func (s *ORSet) Remove(a *any.Any) {
	h := s.hashAny(a)
	if _, exists := s.value[h]; exists {
		if len(s.value) == 1 {
			s.Clear()
		} else {
			delete(s.value, h)
			if _, has := s.added[h]; has {
				delete(s.added, h)
			} else {
				s.removed[h] = a
			}
		}
	}
}

func (s *ORSet) Clear() {
	s.cleared = true
	s.added = make(map[uint64]*any.Any)
	s.removed = make(map[uint64]*any.Any)
	s.value = make(map[uint64]*any.Any)
}

func (s ORSet) Value() []*any.Any {
	val := make([]*any.Any, 0, s.Size())
	for _, v := range s.value {
		val = append(val, v)
	}
	return val
}

func (s ORSet) Added() []*any.Any {
	val := make([]*any.Any, 0, len(s.added))
	for _, v := range s.added {
		val = append(val, v)
	}
	return val
}

func (s ORSet) Removed() []*any.Any {
	val := make([]*any.Any, 0, len(s.removed))
	for _, v := range s.removed {
		val = append(val, v)
	}
	return val
}

func (s *ORSet) State() *protocol.ORSetState {
	return &protocol.ORSetState{
		Items: s.Value(),
	}
}

func (s *ORSet) Delta() *protocol.ORSetDelta {
	return &protocol.ORSetDelta{
		Added:   s.Added(),
		Removed: s.Removed(),
		Cleared: s.cleared,
	}
}

func (s *ORSet) HasDelta() bool {
	return s.cleared == true || len(s.added) > 0 || len(s.removed) > 0
}

func (s *ORSet) ResetDelta() {
	s.cleared = false
	s.added = make(map[uint64]*any.Any)
	s.removed = make(map[uint64]*any.Any)
}

func (s *ORSet) ApplyState(state *protocol.ORSetState) {
	s.value = make(map[uint64]*any.Any)
	if len(state.GetItems()) != 0 {
		for _, a := range state.Items {
			s.value[s.hashAny(a)] = a
		}
	}
}

func (s *ORSet) ApplyDelta(delta *protocol.ORSetDelta) {
	if delta.Cleared {
		s.value = make(map[uint64]*any.Any)
	}
	for _, r := range delta.Removed {
		h := s.hashAny(r)
		if _, has := s.value[h]; has {
			delete(s.value, h)
		}
	}
	for _, a := range delta.Added {
		h := s.hashAny(a)
		if _, has := s.value[h]; !has {
			s.value[h] = a
		}
	}
}
