package extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// The read loop always stays available to route Host responses while an
// extension callback is waiting for model.generate.
func (s *Server) hostCall(ctx context.Context, method string, params any, target any) error {
	s.mu.Lock()
	if s.send == nil {
		s.mu.Unlock()
		return errors.New("affectweave: host unavailable")
	}
	s.requestSequence++
	id := fmt.Sprintf("aw:%d", s.requestSequence)
	ch := make(chan frame, 1)
	if s.pending == nil {
		s.pending = map[string]chan frame{}
	}
	s.pending[id] = ch
	send := s.send
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.pending, id); s.mu.Unlock() }()
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	idRaw, _ := json.Marshal(id)
	if err = send(frame{JSONRPC: "2.0", ID: idRaw, Method: method, Params: raw}); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case response := <-ch:
		if response.Error != nil {
			return fmt.Errorf("affectweave: host error %d", response.Error.Code)
		}
		return json.Unmarshal(response.Result, target)
	}
}
func (s *Server) receiveHostResponse(f frame) {
	var id string
	if json.Unmarshal(f.ID, &id) != nil {
		return
	}
	s.mu.Lock()
	ch := s.pending[id]
	s.mu.Unlock()
	if ch != nil {
		select {
		case ch <- f:
		default:
		}
	}
}
