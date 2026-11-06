// Package extension implements only the FreeAgent NDJSON adapter. All
// character rules and persistence are independent from the transport.
package extension

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/endview/affectweave/internal/affect"
	"github.com/endview/affectweave/internal/storage"
)

const ID = "cc.endview.affectweave"

type frame struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type Adopted struct {
	ID    string    `json:"id"`
	Actor string    `json:"actor_id"`
	Text  string    `json:"text"`
	At    time.Time `json:"occurred_at"`
}
type Identity struct {
	Core         string    `json:"core_id"`
	Space        string    `json:"space_id"`
	Conversation string    `json:"conversation_id"`
	Run          string    `json:"run_id"`
	Inputs       []Adopted `json:"adopted_inputs"`
}
type Params struct {
	Protocol   string    `json:"protocol"`
	Invocation string    `json:"invocation"`
	Identity   *Identity `json:"identity"`
	Settings   struct {
		Role     string `json:"role_id"`
		MaxBytes int    `json:"context_bytes"`
		Analysis bool   `json:"analysis_enabled"`
	} `json:"settings"`
	Variant     string         `json:"variant"`
	Output      string         `json:"output"`
	Status      string         `json:"status"`
	Scope       affect.Scope   `json:"scope"`
	OperationID string         `json:"operation_id"`
	Digest      string         `json:"digest"`
	Change      storage.Change `json:"change"`
	Facts       []RunFacts     `json:"facts"`
	PauseID     string         `json:"pause_id"`
	Until       time.Time      `json:"until"`
	Cursor      string         `json:"cursor"`
}
type RunFacts struct {
	Identity   *Identity         `json:"identity"`
	Status     string            `json:"status"`
	Output     string            `json:"output"`
	Updated    time.Time         `json:"updated_at"`
	Deliveries []json.RawMessage `json:"deliveries"`
}
type Server struct {
	Store       *storage.Store
	writeMu     sync.Mutex
	mu          sync.Mutex
	active      map[string]context.CancelFunc
	writes      int
	pausedUntil time.Time
	pauseID     string
}

func (s *Server) beginWrite() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Now().Before(s.pausedUntil) {
		return errors.New("affectweave: writes paused for bounded backup")
	}
	s.writes++
	return nil
}
func (s *Server) endWrite() { s.mu.Lock(); s.writes--; s.mu.Unlock() }
func (s *Server) pause(ctx context.Context, id string, until time.Time) error {
	if id == "" || until.Before(time.Now()) || until.After(time.Now().Add(2*time.Minute)) {
		return affect.ErrInvalid
	}
	s.mu.Lock()
	if time.Now().Before(s.pausedUntil) && s.pauseID != id {
		s.mu.Unlock()
		return errors.New("affectweave: already paused")
	}
	s.pauseID = id
	s.pausedUntil = until
	s.mu.Unlock()
	for {
		s.mu.Lock()
		active := s.writes
		s.mu.Unlock()
		if active == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			s.mu.Lock()
			if s.pauseID == id {
				s.pauseID = ""
				s.pausedUntil = time.Time{}
			}
			s.mu.Unlock()
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}
func (s *Server) handle(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	var p Params
	if json.Unmarshal(raw, &p) != nil {
		return nil, affect.ErrInvalid
	}
	if method == "initialize" {
		if p.Protocol != "freeagent-ext/1" {
			return nil, affect.ErrInvalid
		}
		return map[string]any{"id": ID, "version": affect.Version, "provides": []string{"context", "state"}, "events": []string{"turn.finished"}, "workflows": []string{}}, nil
	}
	if method == "shutdown" {
		return map[string]bool{"ok": true}, nil
	}
	if s.Store == nil {
		return nil, errors.New("affectweave: explicitly initialize PostgreSQL storage first")
	}
	switch method {
	case "state.sources":
		return s.Store.Sources(ctx, p.Cursor)
	case "state.pause_status":
		s.mu.Lock()
		defer s.mu.Unlock()
		return map[string]any{"paused": s.pauseID == p.PauseID && time.Now().Before(s.pausedUntil) && s.writes == 0, "pause_id": s.pauseID, "until": s.pausedUntil}, nil
	case "state.read":
		if p.Scope.Core != s.Store.CoreID || p.Scope.Space == "" {
			return nil, storage.ErrScope
		}
		roles, err := s.Store.Roles(ctx, p.Scope.Space)
		if err != nil {
			return nil, err
		}
		choices := []map[string]string{}
		for _, r := range roles {
			choices = append(choices, map[string]string{"id": r.ID, "label": r.Persona.Name})
		}
		result := map[string]any{"status": "ready", "storage": "postgresql", "version": affect.Version, "core_id": s.Store.CoreID, "roles": roles, "choices": choices, "state": nil}
		if p.Scope.Role != "" {
			state, err := s.Store.Read(ctx, p.Scope)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			if err == nil {
				result["state"] = state
				result["inputs"], err = s.Store.InputReferences(ctx, p.Scope)
				if err != nil {
					return nil, err
				}
				result["memories"], err = s.Store.Memories(ctx, p.Scope)
				if err != nil {
					return nil, err
				}
				result["facts"], err = s.Store.Facts(ctx, p.Scope, 50)
				if err != nil {
					return nil, err
				}
			}
		}
		return result, nil
	case "state.preview":
		var c struct {
			ID string `json:"id"`
			storage.Change
		}
		if strictJSON(raw, &c) != nil {
			return nil, affect.ErrInvalid
		}
		return s.Store.Preview(ctx, c.Change)
	case "state.apply":
		var envelope struct {
			OperationID string `json:"operation_id"`
			Digest      string `json:"digest"`
			Change      struct {
				ID string `json:"id"`
				storage.Change
			} `json:"change"`
		}
		if strictJSON(raw, &envelope) != nil {
			return nil, affect.ErrInvalid
		}
		if err := s.beginWrite(); err != nil {
			return nil, err
		}
		defer s.endWrite()
		return s.Store.Apply(ctx, envelope.OperationID, envelope.Digest, envelope.Change.Change)
	case "state.result":
		return s.Store.Operation(ctx, p.OperationID, p.Digest)
	case "state.pause":
		if err := s.pause(ctx, p.PauseID, p.Until); err != nil {
			return nil, err
		}
		return map[string]any{"paused": true, "pause_id": p.PauseID, "until": p.Until}, nil
	case "state.resume":
		s.mu.Lock()
		if s.pauseID == "" {
			s.mu.Unlock()
			return map[string]bool{"paused": false}, nil
		}
		if s.pauseID != p.PauseID {
			s.mu.Unlock()
			return nil, affect.ErrInvalid
		}
		s.pausedUntil = time.Time{}
		s.pauseID = ""
		s.mu.Unlock()
		return map[string]bool{"paused": false}, nil
	case "context.prepare", "turn.finished", "facts.sync":
		if p.Identity == nil || p.Identity.Core != s.Store.CoreID || p.Identity.Run == "" || len(p.Identity.Inputs) == 0 || len(p.Identity.Inputs) > 128 {
			return nil, affect.ErrInvalid
		}
		role := p.Variant
		if role == "" {
			role = p.Settings.Role
		}
		if role == "" {
			if method == "context.prepare" {
				return map[string]any{"blocks": []any{}}, nil
			}
			return map[string]any{"activity": map[string]any{"summary": "No character selected"}}, nil
		}
		if err := s.beginWrite(); err != nil {
			return nil, err
		}
		defer s.endWrite()
		var state affect.State
		var scope affect.Scope
		for _, facts := range p.Facts {
			if err := s.syncFacts(ctx, role, facts); err != nil {
				return nil, err
			}
		}
		for _, in := range p.Identity.Inputs {
			scope = affect.Scope{Core: p.Identity.Core, Space: p.Identity.Space, Role: role, Actor: in.Actor}
			var err error
			state, err = s.Store.Adopt(ctx, scope, storage.Input{ID: in.ID, RunID: p.Identity.Run, Text: in.Text, At: in.At})
			if err != nil {
				return nil, err
			}
		}
		if method == "context.prepare" {
			memories, err := s.Store.Memories(ctx, scope)
			if err != nil {
				return nil, err
			}
			limit := p.Settings.MaxBytes
			if limit == 0 {
				limit = 6000
			}
			packet, err := affect.Compile(state, memories, []string{p.Identity.Inputs[len(p.Identity.Inputs)-1].ID}, limit)
			if err != nil {
				return nil, err
			}
			return map[string]any{"blocks": []any{map[string]any{"id": "character", "text": packet.Text, "placement": "tail", "label": state.Persona.Name, "priority": 40}}}, nil
		}
		if method == "turn.finished" {
			body, _ := json.Marshal(map[string]any{"run_id": p.Identity.Run, "status": p.Status, "output": p.Output, "evidence_level": "L0", "delivery": "not_observed"})
			if err := s.Store.RecordFact(ctx, scope, storage.Fact{ID: "generated:" + p.Identity.Run + ":" + affect.Hash(body), Kind: "generated", Body: body, At: state.Updated}); err != nil {
				return nil, err
			}
		}
		return map[string]any{"activity": map[string]any{"summary": "Character facts recorded; persistent proposals require confirmation"}}, nil
	default:
		return nil, errors.New("affectweave: unsupported method")
	}
}

func strictJSON(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return affect.ErrInvalid
	}
	return nil
}
func (s *Server) syncFacts(ctx context.Context, role string, f RunFacts) error {
	if f.Identity == nil || f.Identity.Core != s.Store.CoreID || len(f.Identity.Inputs) == 0 {
		return affect.ErrInvalid
	}
	for _, in := range f.Identity.Inputs {
		scope := affect.Scope{Core: f.Identity.Core, Space: f.Identity.Space, Role: role, Actor: in.Actor}
		if _, err := s.Store.Adopt(ctx, scope, storage.Input{ID: in.ID, RunID: f.Identity.Run, Text: in.Text, At: in.At}); err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]any{"run_id": f.Identity.Run, "status": f.Status, "output": f.Output, "evidence_level": "L0"})
		if err := s.Store.RecordFact(ctx, scope, storage.Fact{ID: "generated:" + f.Identity.Run + ":" + affect.Hash(body), Kind: "generated", Body: body, At: f.Updated}); err != nil {
			return err
		}
		for _, delivery := range f.Deliveries {
			var d struct {
				ID    string `json:"receipt_id"`
				State string `json:"state"`
			}
			if json.Unmarshal(delivery, &d) != nil || d.ID == "" {
				return affect.ErrInvalid
			}
			if err := s.Store.RecordFact(ctx, scope, storage.Fact{ID: d.ID, Kind: "delivery", Body: delivery, At: f.Updated}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) Serve(parent context.Context, input io.Reader, output io.Writer) error {
	ctx, cancelAll := context.WithCancel(parent)
	defer cancelAll()
	s.active = map[string]context.CancelFunc{}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	encoder := json.NewEncoder(output)
	slots := make(chan struct{}, 4)
	var work sync.WaitGroup
	respond := func(id json.RawMessage, result any, err error) {
		reply := frame{JSONRPC: "2.0", ID: id}
		if err != nil {
			message := "affectweave: request failed; inspect storage and input"
			code := -32000
			switch {
			case errors.Is(err, storage.ErrConflict):
				message = storage.ErrConflict.Error()
				code = -32010
			case errors.Is(err, storage.ErrScope):
				message = storage.ErrScope.Error()
			case errors.Is(err, storage.ErrOperation):
				message = storage.ErrOperation.Error()
			case errors.Is(err, affect.ErrInvalid):
				message = affect.ErrInvalid.Error()
			case errors.Is(err, context.Canceled):
				message = "cancelled"
				code = -32800
			}
			reply.Error = &rpcError{Code: code, Message: message}
		} else {
			reply.Result, _ = json.Marshal(result)
		}
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
		_ = encoder.Encode(reply)
	}
	defer func() { cancelAll(); work.Wait() }()
	for scanner.Scan() {
		var f frame
		if json.Unmarshal(scanner.Bytes(), &f) != nil || f.JSONRPC != "2.0" {
			respond(json.RawMessage("null"), nil, affect.ErrInvalid)
			continue
		}
		var p Params
		_ = json.Unmarshal(f.Params, &p)
		if f.Method == "$/cancel" {
			s.mu.Lock()
			if cancel := s.active[p.Invocation]; cancel != nil {
				cancel()
			}
			s.mu.Unlock()
			continue
		}
		if len(f.ID) == 0 {
			continue
		}
		if f.Method == "shutdown" {
			respond(f.ID, map[string]bool{"ok": true}, nil)
			return nil
		}
		select {
		case slots <- struct{}{}:
		default:
			respond(f.ID, nil, errors.New("busy"))
			continue
		}
		key := p.Invocation
		if key == "" {
			key = "rpc:" + string(f.ID)
		}
		callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		s.mu.Lock()
		if s.active[key] != nil {
			s.mu.Unlock()
			cancel()
			<-slots
			respond(f.ID, nil, affect.ErrInvalid)
			continue
		}
		s.active[key] = cancel
		s.mu.Unlock()
		work.Add(1)
		go func(f frame, key string) {
			defer work.Done()
			defer func() { s.mu.Lock(); delete(s.active, key); s.mu.Unlock(); cancel(); <-slots }()
			result, err := s.handle(callCtx, f.Method, f.Params)
			respond(f.ID, result, err)
		}(f, key)
	}
	if scanner.Err() != nil {
		return errors.New("affectweave: invalid stdio frame or input failed")
	}
	return nil
}
func Main() int {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(affect.Version)
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	store, err := storage.Open(ctx, os.Getenv("FREEAGENT_EXTENSION_PG_FILE"))
	cancel()
	if err != nil {
		_, _ = io.WriteString(os.Stderr, "AffectWeave storage unavailable; initialize restricted extension PostgreSQL storage.\n")
		return 1
	}
	defer store.Close()
	s := &Server{Store: store}
	if err = s.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		_, _ = io.WriteString(os.Stderr, "AffectWeave transport stopped.\n")
		return 1
	}
	return 0
}
