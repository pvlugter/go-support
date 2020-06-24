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
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/cloudstateio/go-support/cloudstate/protocol"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Entity captures an Entity with its ServiceName.
// It is used to be registered as an CRDT entity on a Cloudstate instance.
type Entity struct {
	// ServiceName is the fully qualified name of the service that implements this entities interface.
	// Setting it is mandatory.
	ServiceName ServiceName
	// EntityFunc creates a new entity.
	EntityFunc func(id EntityId) interface{}
	// SetFunc is a function that sets the ...
	SetFunc func(c *Context, crdt CRDT)
	// DefaultFunc is a factory function to create the CRDT to be used for this entity.
	DefaultFunc func(c *Context) CRDT
	CommandFunc func(entity interface{}, ctx *CommandContext, name string, msg interface{}) (*any.Any, error)
}

type Server struct {
	// mu protects the map below.
	mu sync.RWMutex
	// entities has descriptions of entities registered by service names
	entities map[ServiceName]*Entity
}

// NewServer returns an initialized Server
func NewServer() *Server {
	return &Server{
		entities: make(map[ServiceName]*Entity),
	}
}

// CrdtEntities can be registered to a server that handles crdt entities by a ServiceName.
// Whenever a internalCRDT.Server receives an CrdInit for an instance of a crdt entity identified by its
// EntityId and a ServiceName, the internalCRDT.Server handles such entities through their lifecycle.
// The handled entities value are captured by a context that is held fo each of them.
func (s *Server) Register(e *Entity) error {
	if e.EntityFunc == nil {
		return fmt.Errorf("the entity has to define an EntityFunc but did not")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entities[e.ServiceName]; exists {
		return fmt.Errorf("an entity with service name: %s is already registered", e.ServiceName)
	}
	s.entities[e.ServiceName] = e
	return nil
}

// After invoking handle, the first message sent will always be a CrdtInit message, containing the entity ID, and,
// if it exists or is available, the current value of the entity. After that, one or more commands may be sent,
// as well as deltas as they arrive, and the entire value if either the entity is created, or the proxy wishes the
// user function to replace its entire value.
// The user function must respond with one reply per command in. They do not necessarily have to be sent in the same
// order that the commands were sent, the command ID is used to correlate commands to replies.
func (s *Server) Handle(stream protocol.Crdt_HandleServer) error {
	defer func() {
		if r := recover(); r != nil {
			_ = sendFailure(fmt.Errorf("CrdtServer.Handle panic-ked with: %v", r), stream)
			panic(r)
		}
	}()
	if err := s.handle(stream); err != nil {
		if status.Code(err) == codes.Canceled {
			return err
		}
		log.Print(err)
		if sendErr := sendFailure(err, stream); sendErr != nil {
			log.Print(sendErr)
		}
		return status.Error(codes.Aborted, err.Error())
	}
	return nil
}

func (s *Server) handle(stream protocol.Crdt_HandleServer) error {
	first, err := stream.Recv()
	if err == io.EOF { // the stream has ended
		return nil
	}
	if err == context.Canceled {
		return nil
	}
	if err != nil {
		return err
	}

	runner := &runner{stream: stream}
	switch m := first.GetMessage().(type) {
	case *protocol.CrdtStreamIn_Init:
		// first, always a CrdtInit message must be received.
		if err = s.handleInit(m.Init, runner); err != nil {
			return fmt.Errorf("handling of CrdtInit failed with: %w", err)
		}
	default:
		return fmt.Errorf("a message was received without having an CrdtInit message first: %v", m)
	}
	if runner.context.EntityId == "" { // never happens, but if.
		return fmt.Errorf("a message was received without having a CrdtInit message handled before: %v", first.GetMessage())
	}

	// handle all other messages after a CrdtInit message has been received.
	for {
		if runner.context.deleted || !runner.context.active {
			return nil // TODO: this will close the stream but not tell the proxy why.
		}
		if runner.context.failed != nil {
			// failed means deactivated. we may never get this far.
			return nil
		}
		msg, err := runner.stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch m := msg.GetMessage().(type) {
		case *protocol.CrdtStreamIn_State:
			if err := runner.handleState(m.State); err != nil {
				return err
			}
			if err := runner.handleChange(); err != nil {
				return err
			}
		case *protocol.CrdtStreamIn_Changed:
			if !runner.stateReceived {
				return errors.New("received a CrdtDelta message without having a CrdtState ever received")
			}
			if err := runner.handleDelta(m.Changed); err != nil {
				return err
			}
			if err := runner.handleChange(); err != nil {
				return err
			}
		case *protocol.CrdtStreamIn_Deleted:
			// Delete the entity. May be sent at any time. The user function should clear its value when it receives this.
			// A proxy may decide to terminate the stream after sending this.
			runner.context.Delete()
		case *protocol.CrdtStreamIn_Command:
			// A command, may be sent at any time.
			// The CRDT is allowed to be changed.
			if err := runner.handleCommand(m.Command); err != nil {
				return err
			}
		case *protocol.CrdtStreamIn_StreamCancelled:
			// The CRDT is allowed to be changed.
			if err := runner.handleCancellation(m.StreamCancelled); err != nil {
				return err
			}
			if err := runner.handleChange(); err != nil {
				return err
			}
		case *protocol.CrdtStreamIn_Init:
			return errors.New("duplicate init message for the same entity")
		case nil:
			return errors.New("empty message received")
		default:
			return fmt.Errorf("unknown message received: %v", msg.GetMessage())
		}
	}
}

func (s *Server) handleInit(init *protocol.CrdtInit, r *runner) error {
	serviceName := ServiceName(init.GetServiceName())
	s.mu.RLock()
	entity, exists := s.entities[serviceName]
	s.mu.RUnlock()
	if !exists {
		return fmt.Errorf("received a command for an unknown crdt service: %v", serviceName)
	}
	if entity.EntityFunc == nil {
		return fmt.Errorf("entity.EntityFunc not defined: %v", serviceName)
	}
	id := EntityId(init.GetEntityId())
	r.context = &Context{
		EntityId:    id,
		Entity:      entity,
		Instance:    entity.EntityFunc(id),
		active:      true,
		created:     false,
		ctx:         r.stream.Context(), // this context is stable as long as the runner runs
		streamedCtx: make(map[CommandId]*CommandContext),
	}
	// the init msg may have an initial state
	if state := init.GetState(); state != nil {
		if err := r.handleState(state); err != nil {
			return err
		}
	}
	// the user entity can provide a CRDT through a default function if none is set.
	return r.context.initDefault()
}