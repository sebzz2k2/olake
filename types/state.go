package types

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/goccy/go-json"
)

type StateType string

const (
	// Global Type indicates that the connector solely acts on Globally shared state across streams
	GlobalType StateType = "GLOBAL"
	// Streme Type indicates that the connector solely acts on individual stream state
	StreamType StateType = "STREAM"
	// Mixed type indicates that the connector works with a mix of Globally shared and
	// Individual stream state
	MixedType StateType = "MIXED"
)

// TODO: Add validation tags; Write custom unmarshal that triggers validation
// State is a dto for airbyte state serialization
type State struct {
	*sync.Mutex `json:"-"`
	Type        StateType      `json:"type"`
	Global      any            `json:"global,omitempty"`
	Streams     []*StreamState `json:"streams,omitempty"`
}

var (
	ErrStateMissing       = errors.New("stream missing from state")
	ErrStateCursorMissing = errors.New("cursor field missing from state")
)

func (s *State) SetType(typ StateType) {
	s.Type = typ
}

// func (s *State) Add(stream, namespace string, field string, value any) {
// 	s.Streams = append(s.Streams, &StreamState{
// 		Stream:    stream,
// 		Namespace: namespace,
// 		State: map[string]any{
// 			field: value,
// 		},
// 	})
// }

// func (s *State) Get(streamName, namespace string) map[string]any {
// 	for _, stream := range s.Streams {
// 		if stream.Stream == streamName && stream.Namespace == namespace {
// 			return stream.State
// 		}
// 	}

// 	return nil
// }

func (s *State) IsZero() bool {
	return s.Global == nil && len(s.Streams) == 0
}

func (s *State) MarshalJSON() ([]byte, error) {
	if s.IsZero() {
		return nil, nil
	}

	type Alias State
	p := Alias(*s)

	populatedStreams := []*StreamState{}
	for _, stream := range p.Streams {
		if stream.Load() {
			populatedStreams = append(populatedStreams, stream)
		}
	}

	p.Streams = populatedStreams
	return json.Marshal(p)
}

type StreamState struct {
	atomic.Bool `json:"-"` // If State holds some value and should not be excluded during unmarshaling then value true

	Stream    string `json:"stream"`
	Namespace string `json:"namespace"`
	State     sync.Map
}

// MarshalJSON custom marshaller to handle sync.Map encoding
func (s *StreamState) MarshalJSON() ([]byte, error) {
	// Create a map to temporarily store data for JSON marshalling
	stateMap := make(map[string]interface{})
	s.State.Range(func(key, value interface{}) bool {
		strKey, ok := key.(string)
		if !ok {
			return false
		}
		stateMap[strKey] = value
		return true
	})

	// Create an alias to avoid infinite recursion
	type Alias StreamState
	return json.Marshal(&struct {
		*Alias
		State map[string]interface{} `json:"state"`
	}{
		Alias: (*Alias)(s),
		State: stateMap,
	})
}

// UnmarshalJSON custom unmarshaller to handle sync.Map decoding
func (s *StreamState) UnmarshalJSON(data []byte) error {
	// Create a temporary structure to unmarshal JSON into
	type Alias StreamState
	aux := &struct {
		*Alias
		State map[string]interface{} `json:"state"`
	}{
		Alias: (*Alias)(s),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Populate sync.Map with the data from temporary map
	for key, value := range aux.State {
		s.State.Store(key, value)
	}

	return nil
}

func NewGlobalState[T GlobalState](state T) *Global[T] {
	return &Global[T]{
		State:   state,
		Streams: NewSet[string](),
	}
}

type GlobalState interface {
	IsEmpty() bool
}

type Global[T GlobalState] struct {
	// Global State shared by streams
	State T `json:"state"`
	// Attaching Streams to Global State helps in recognizing the tables that the state belongs to.
	//
	// This results in helping connector determine what streams were synced during the last sync in
	// Group read. and also helps connectors to migrate from incremental to GroupRead without the need to
	// full load with the help of using cursor value and field as recovery cursor for GroupRead
	Streams *Set[string] `json:"streams"`
}

func (g *Global[T]) MarshalJSON() ([]byte, error) {
	if any(g.State).(GlobalState).IsEmpty() {
		return json.Marshal(nil)
	}

	type Alias Global[T]
	p := Alias(*g)

	return json.Marshal(p)
}

func (g *Global[T]) UnmarshalJSON(data []byte) error {
	// Define a type alias to avoid recursion
	type Alias Global[T]

	// Create a temporary alias value to unmarshal into
	var temp Alias

	temp.Streams = NewSet[string]()

	err := json.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	*g = Global[T](temp)
	return nil
}
