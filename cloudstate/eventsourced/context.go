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

package eventsourced

type Context struct {
	// EntityId
	EntityId EntityId
	// Instance is an instance of the registered entity.
	Instance interface{}
	// EventSourcedEntity describes the instance hold by the EntityInstance.
	EventSourcedEntity *Entity

	EventEmitter
	failed        error
	active        bool
	eventSequence int64
}

func (c *Context) Failed(err error) {
	c.failed = err
}

func (c *Context) shouldSnapshot() bool {
	return c.eventSequence >= c.EventSourcedEntity.SnapshotEvery
}

func (c *Context) resetSnapshotEvery() {
	c.eventSequence = 0
}
