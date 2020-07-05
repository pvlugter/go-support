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

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/cloudstateio/go-support/cloudstate/protocol"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const snapshotEveryDefault = 100

// Server is the implementation of the Server server API for EventSourced service.
type Server struct {
	// mu protects the map below.
	mu sync.RWMutex
	// entities are indexed by their service name.
	entities map[ServiceName]*Entity
}

// NewServer returns a new eventsourced server.
func NewServer() *Server {
	return &Server{
		entities: make(map[ServiceName]*Entity),
	}
}

func (s *Server) Register(e *Entity) error {
	if e.EntityFunc == nil {
		return fmt.Errorf("the entity has to define an EntityFunc but did not")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entities[e.ServiceName]; exists {
		return fmt.Errorf("an entity with service name: %s is already registered", e.ServiceName)
	}
	e.SnapshotEvery = snapshotEveryDefault
	s.entities[e.ServiceName] = e
	return nil
}

// Handle handles the stream. One stream will be established per active entity.
// Once established, the first message sent will be Init, which contains the entity ID, and,
// if the entity has previously persisted a snapshot, it will contain that snapshot. It will
// then send zero to many event messages, one for each event previously persisted. The entity
// is expected to apply these to its state in a deterministic fashion. Once all the events
// are sent, one to many commands are sent, with new commands being sent as new requests for
// the entity come in. The entity is expected to reply to each command with exactly one reply
// message. The entity should reply in order, and any events that the entity requests to be
// persisted the entity should handle itself, applying them to its own state, as if they had
// arrived as events when the event stream was being replayed on load.
//
// Error handling is done so that any error returned, triggers the stream to be closed.
// If an error is a client failure, a ClientAction_Failure is sent with a command id set
// if provided by the error. If an error is a protocol failure or any other error, a
// EventSourcedStreamOut_Failure is sent. A protocol failure might provide a command id to
// be included.
func (s *Server) Handle(stream protocol.EventSourced_HandleServer) error {
	defer func() {
		if r := recover(); r != nil {
			// on panic we try to tell the proxy and panic again.
			_ = sendFailure(fmt.Errorf("Server.Handle panic-ked with: %v", r), stream)
			// there are two ways to do this
			// a) report and close the stream and let others run
			// b) report and panic and therefore crash the program
			// how can we decide that the panic keeps the user function in
			// a consistent state. this one occasion could be perfectly ok
			// to crash, but thousands of other keep running. why get them all down?
			// so then there is the presumption that a panic is truly exceptional
			// and we can't be sure about anything to be safe after one.
			// the proxy is well prepared for this, it is able to re-establish state
			// and also isolate the erroneous entity type from others.
			panic(r)
		}
	}()

	// for any error we get, we send a protocol.Failure and close the stream.
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

func (s *Server) handle(server protocol.EventSourced_HandleServer) error {
	first, err := server.Recv()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	runner := &runner{stream: server}
	switch m := first.GetMessage().(type) {
	case *protocol.EventSourcedStreamIn_Init:
		if err := s.handleInit(m.Init, runner); err != nil {
			return err
		}
	default:
		return fmt.Errorf("a message was received without having a EventSourcedInit message handled before: %v", first.GetMessage())
	}
	for {
		if runner.context.failed != nil {
			// failed means deactivated. we may never get this far.
			// context.failed should have been sent as a client reply failure
			return fmt.Errorf("failed context was not reported: %w", runner.context.failed)
		}
		if runner.context.active == false {
			// TODO: what do we report here
			// see: https://github.com/cloudstateio/cloudstate/pull/119#discussion_r444851439
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
		case *protocol.EventSourcedStreamIn_Command:
			err := runner.handleCommand(m.Command)
			if err == nil {
				continue
			}
			if err := sendFailure(err, runner.stream); err != nil {
				return err
			}
		case *protocol.EventSourcedStreamIn_Event:
			if err := runner.handleEvent(m.Event); err != nil {
				return err
			}
		case *protocol.EventSourcedStreamIn_Init:
			return errors.New("duplicate init message for the same entity")
		case nil:
			return errors.New("empty message received")
		default:
			return fmt.Errorf("unknown message received: %v", msg.GetMessage())
		}
	}
}

func (s *Server) handleInit(init *protocol.EventSourcedInit, r *runner) error {
	serviceName := ServiceName(init.GetServiceName())
	s.mu.RLock()
	entity, exists := s.entities[serviceName]
	s.mu.RUnlock()
	if !exists {
		return fmt.Errorf("received a command for an unknown eventsourced service: %v", serviceName)
	}
	if entity.EntityFunc == nil {
		return fmt.Errorf("entity.EntityFunc not defined: %v", serviceName)
	}

	id := EntityId(init.GetEntityId())
	r.context = &Context{
		EntityId:           id,
		EventSourcedEntity: entity,
		Instance:           entity.EntityFunc(id),
		EventEmitter:       newEmitter(),
		active:             true,
		eventSequence:      0,
		ctx:                r.stream.Context(),
	}
	if snapshot := init.GetSnapshot(); snapshot != nil {
		if err := r.handleInitSnapshot(snapshot); err != nil {
			return err
		}
	}
	r.subscribeEvents()
	return nil
}
