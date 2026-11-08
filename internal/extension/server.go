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
	FeedbackSource string    `json:"feedback_source,omitempty"`
	ID             string    `json:"id"`
	Actor          string    `json:"actor_id"`
	Text           string    `json:"text"`
	At             time.Time `json:"occurred_at"`
}
type Identity struct {
	Trigger      *trigger  `json:"trigger,omitempty"`
	Core         string    `json:"core_id"`
	Space        string    `json:"space_id"`
	Conversation string    `json:"conversation_id"`
	Run          string    `json:"run_id"`
	Inputs       []Adopted `json:"adopted_inputs"`
}
type Params struct {
	Deadline     time.Time `json:"deadline"`
	Capabilities []string  `json:"capabilities"`
	Protocol     string    `json:"protocol"`
	StateVersion int       `json:"state_version"`
	Invocation   string    `json:"invocation"`
	Identity     *Identity `json:"identity"`
	Settings     struct {
		Role             string `json:"role_id"`
		MaxBytes         int    `json:"context_bytes"`
		Analysis         bool   `json:"analysis_enabled"`
		AnalysisDeadline int    `json:"analysis_deadline_ms"`
		ReplyMode        string `json:"reply_mode"`
		ReplyParts       int    `json:"max_text_parts"`
		Expression       bool   `json:"expression_enabled"`
		Followup         bool   `json:"fixed_followup_enabled"`
	} `json:"settings"`
	Delivery *struct {
		Reply        bool                   `json:"reply"`
		RuleIDs      []string               `json:"rule_ids"`
		WakeRuleIDs  []string               `json:"wake_rule_ids"`
		Resources    []affect.ReplyResource `json:"resources"`
		Reactions    []string               `json:"reactions"`
		MaxTextParts int                    `json:"max_text_parts"`
	} `json:"delivery"`
	Variant     string         `json:"variant"`
	Output      string         `json:"output"`
	Status      string         `json:"status"`
	Scope       stateScope     `json:"scope"`
	OperationID string         `json:"operation_id"`
	Digest      string         `json:"digest"`
	Change      storage.Change `json:"change"`
	Facts       []RunFacts     `json:"facts"`
	PauseID     string         `json:"pause_id"`
	Until       time.Time      `json:"until"`
	Cursor      string         `json:"cursor"`
}
type RunFacts struct {
	Variant    string            `json:"variant,omitempty"`
	Settings   map[string]any    `json:"settings,omitempty"`
	Identity   *Identity         `json:"identity"`
	Status     string            `json:"status"`
	Output     string            `json:"output"`
	Updated    time.Time         `json:"updated_at"`
	Deliveries []json.RawMessage `json:"deliveries"`
}
type Server struct {
	send            func(frame) error
	pending         map[string]chan frame
	requestSequence uint64
	Store           *storage.Store
	writeMu         sync.Mutex
	mu              sync.Mutex
	active          map[string]context.CancelFunc
	writes          int
	pausedUntil     time.Time
	pauseID         string
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
		if p.Protocol != "freeagent-ext/1" || p.StateVersion != 2 {
			return nil, affect.ErrInvalid
		}
		for _, required := range []string{"context_analysis", "scheduled_generation"} {
			found := false
			for _, capability := range p.Capabilities {
				found = found || capability == required
			}
			if !found {
				return nil, fmt.Errorf("affectweave: host capability %s required", required)
			}
		}
		return map[string]any{"id": ID, "version": affect.Version, "state_version": 2, "provides": []string{"context", "state", "delivery"}, "events": []string{"turn.finished", "delivery.check"}, "workflows": []string{}, "capabilities": []string{"context_analysis", "scheduled_generation"}}, nil
	}
	if method == "shutdown" {
		return map[string]bool{"ok": true}, nil
	}
	if s.Store == nil {
		return nil, errors.New("affectweave: explicitly initialize PostgreSQL storage first")
	}
	switch method {
	case "delivery.check":
		return s.checkDelivery(ctx, raw)
	case "wake.evaluate":
		return s.evaluateWake(ctx, raw)
	case "state.sources":
		return s.Store.Sources(ctx, p.Cursor)
	case "state.pause_status":
		s.mu.Lock()
		defer s.mu.Unlock()
		return map[string]any{"paused": s.pauseID == p.PauseID && time.Now().Before(s.pausedUntil) && s.writes == 0, "pause_id": s.pauseID, "until": s.pausedUntil}, nil
	case "state.read":
		scope := p.Scope.internal()
		if scope.Core != s.Store.CoreID || scope.Space == "" {
			return nil, storage.ErrScope
		}
		roles, err := s.Store.Roles(ctx, scope.Space)
		if err != nil {
			return nil, err
		}
		choices := []map[string]string{}
		for _, r := range roles {
			choices = append(choices, map[string]string{"id": r.ID, "label": r.Persona.Name})
		}
		result := map[string]any{"status": "ready", "storage": "postgresql", "version": affect.Version, "core_id": s.Store.CoreID, "roles": roles, "choices": choices, "state": nil}
		if scope.Role != "" {
			state, err := s.Store.Read(ctx, scope)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			if err == nil {
				result["versions"], err = s.Store.Versions(ctx, scope, 0)
				if err != nil {
					return nil, err
				}
				result["evidence"], err = s.Store.PersonaEvidence(ctx, scope)
				if err != nil {
					return nil, err
				}
				result["state"] = state
				result["analyses"], err = s.Store.Analyses(ctx, scope)
				if err != nil {
					return nil, err
				}
				result["inputs"], err = s.Store.InputReferences(ctx, scope)
				if err != nil {
					return nil, err
				}
				result["memories"], err = s.Store.Memories(ctx, scope)
				if err != nil {
					return nil, err
				}
				result["facts"], err = s.Store.Facts(ctx, scope, 50)
				if err != nil {
					return nil, err
				}
			}
		}
		return result, nil
	case "state.preview":
		var wire stateChange
		if strictJSON(raw, &wire) != nil {
			return nil, affect.ErrInvalid
		}
		c, err := wire.internal()
		if err != nil {
			return nil, err
		}
		return s.Store.Preview(ctx, c)
	case "state.apply":
		var envelope struct {
			OperationID string      `json:"operation_id"`
			Digest      string      `json:"digest"`
			Change      stateChange `json:"change"`
		}
		if strictJSON(raw, &envelope) != nil {
			return nil, affect.ErrInvalid
		}
		c, err := envelope.Change.internal()
		if err != nil {
			return nil, err
		}
		if err := s.beginWrite(); err != nil {
			return nil, err
		}
		defer s.endWrite()
		return s.Store.Apply(ctx, envelope.OperationID, envelope.Digest, c)
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
			if method == "turn.finished" {
				return finishedResponse(p, "No character selected; optional reply expression only")
			}
			return map[string]any{"activity": map[string]any{"summary": "No character selected"}}, nil
		}
		if err := s.beginWrite(); err != nil {
			return nil, err
		}
		defer s.endWrite()
		var state affect.State
		var frozen storage.FrozenInput
		var scope affect.Scope
		for _, facts := range p.Facts {
			if err := s.syncFacts(ctx, role, facts); err != nil {
				return nil, err
			}
		}
		for _, in := range p.Identity.Inputs {
			scope = affect.Scope{Core: p.Identity.Core, Space: p.Identity.Space, Role: role, Actor: in.Actor}
			var err error
			frozen, err = s.Store.AdoptFrozen(ctx, scope, storage.Input{ID: in.ID, RunID: p.Identity.Run, Text: in.Text, At: in.At, FeedbackSource: in.FeedbackSource}, storage.InputOptions{MaxBytes: p.Settings.MaxBytes, Analysis: p.Settings.Analysis, DeadlineMS: p.Settings.AnalysisDeadline})
			state = frozen.State
			if err != nil {
				return nil, err
			}
		}
		if method == "context.prepare" {
			if frozen.Options.Analysis {
				var err error
				frozen, err = s.analyze(ctx, p, scope, frozen)
				if err != nil {
					return nil, err
				}
			}
			packet := frozen.Context
			return map[string]any{"blocks": []any{map[string]any{"id": "character", "text": packet.Text, "placement": "tail", "label": state.Persona.Name, "priority": 40}}}, nil
		}
		if method == "turn.finished" {
			body, _ := json.Marshal(map[string]any{"run_id": p.Identity.Run, "status": p.Status, "output": p.Output, "evidence_level": "L0", "delivery": "not_observed"})
			if err := s.Store.RecordFact(ctx, scope, storage.Fact{ID: "generated:" + p.Identity.Run + ":" + affect.Hash(body), Kind: "generated", Body: body, At: state.Updated}); err != nil {
				return nil, err
			}
			return s.finishExpressions(ctx, p, frozen, scope)
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
	if f.Settings != nil {
		role = f.Variant
		if role == "" {
			role, _ = f.Settings["role_id"].(string)
		}
	}
	if role == "" {
		return affect.ErrInvalid
	}
	if f.Identity != nil && f.Identity.Trigger != nil {
		t := f.Identity.Trigger
		if t.Extension != ID || t.Kind != "scheduled" || t.Actor == "" || t.Resource == "" {
			return affect.ErrInvalid
		}
		scope := affect.Scope{Core: f.Identity.Core, Space: f.Identity.Space, Role: t.Resource, Actor: t.Actor}
		if _, err := s.Store.Read(ctx, scope); err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]any{"run_id": f.Identity.Run, "status": f.Status, "output": f.Output, "evidence_level": "L0", "trigger": "scheduled"})
		if err := s.Store.RecordFact(ctx, scope, storage.Fact{ID: "wake-generated:" + f.Identity.Run + ":" + affect.Hash(body), Kind: "proactive", Body: body, At: f.Updated}); err != nil {
			return err
		}
		for _, delivery := range f.Deliveries {
			var d struct {
				ID string `json:"receipt_id"`
			}
			if json.Unmarshal(delivery, &d) != nil || d.ID == "" {
				return affect.ErrInvalid
			}
			if err := s.Store.RecordFact(ctx, scope, storage.Fact{ID: d.ID, Kind: "delivery", Body: delivery, At: deliveryTime(delivery, f.Updated)}); err != nil {
				return err
			}
		}
		return nil
	}
	if f.Identity == nil || f.Identity.Core != s.Store.CoreID || len(f.Identity.Inputs) == 0 {
		return affect.ErrInvalid
	}
	for _, in := range f.Identity.Inputs {
		scope := affect.Scope{Core: f.Identity.Core, Space: f.Identity.Space, Role: role, Actor: in.Actor}
		if err := s.Store.EnsureFactSource(ctx, scope, storage.Input{ID: in.ID, RunID: f.Identity.Run, Text: in.Text, At: in.At, FeedbackSource: in.FeedbackSource}); err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]any{"run_id": f.Identity.Run, "status": f.Status, "output": f.Output, "evidence_level": "L0"})
		if err := s.Store.RecordFact(ctx, scope, storage.Fact{ID: "generated:" + f.Identity.Run + ":" + affect.Hash(body), Kind: "generated", Body: body, At: f.Updated}); err != nil {
			return err
		}
		for _, delivery := range f.Deliveries {
			var payload map[string]any
			if json.Unmarshal(delivery, &payload) != nil {
				return affect.ErrInvalid
			}
			payload["run_id"] = f.Identity.Run
			delivery, _ = json.Marshal(payload)
			var d struct {
				ID       string    `json:"receipt_id"`
				State    string    `json:"state"`
				Occurred time.Time `json:"occurred_at"`
			}
			if json.Unmarshal(delivery, &d) != nil || d.ID == "" {
				return affect.ErrInvalid
			}
			if err := s.Store.RecordFact(ctx, scope, storage.Fact{ID: d.ID, Kind: "delivery", Body: delivery, At: deliveryTime(delivery, f.Updated)}); err != nil {
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
	s.mu.Lock()
	s.send = func(f frame) error { s.writeMu.Lock(); defer s.writeMu.Unlock(); return encoder.Encode(f) }
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.send = nil; s.mu.Unlock() }()
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
		if f.Method == "" {
			s.receiveHostResponse(f)
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
		timeout := 20 * time.Second
		if f.Method == "context.prepare" && p.Settings.Analysis {
			ms := p.Settings.AnalysisDeadline
			if ms == 0 {
				ms = 8000
			}
			timeout = time.Duration(max(2000, min(30000, ms)))*time.Millisecond + time.Second
		}
		if f.Method == "wake.evaluate" {
			timeout = 121 * time.Second
		}
		if !p.Deadline.IsZero() {
			timeout = min(timeout, time.Until(p.Deadline))
		}
		callCtx, cancel := context.WithTimeout(ctx, timeout)
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

func deliveryTime(raw []byte, fallback time.Time) time.Time {
	var d struct {
		At time.Time `json:"occurred_at"`
	}
	if json.Unmarshal(raw, &d) == nil && !d.At.IsZero() {
		return d.At
	}
	return fallback
}
