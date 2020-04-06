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

import "github.com/cloudstateio/go-support/cloudstate/protocol"

type CRDT interface {
	State() *protocol.CrdtState
	Delta() *protocol.CrdtDelta
	HasDelta() bool
	// TODO: think about to make this public and let anyone implement their CRDTs
	internalCRDT
}

type internalCRDT interface {
	applyState(*protocol.CrdtState) error
	applyDelta(*protocol.CrdtDelta) error
	resetDelta()
}