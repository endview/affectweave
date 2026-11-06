// Package storage owns only AffectWeave's restricted PostgreSQL database.
package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/endview/affectweave/internal/affect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed schema.sql
var schema string
var ErrConflict = errors.New("affectweave: stale revision")
var ErrScope = errors.New("affectweave: role not authorized for this space")
var ErrOperation = errors.New("affectweave: operation ID reused with different content")

type Connection struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	CoreID   string `json:"core_id"`
}
type Store struct {
	DB     *sql.DB
	CoreID string
}

func Open(ctx context.Context, path string) (*Store, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16384 {
		return nil, errors.New("affectweave: private PG connection file unavailable")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Connection
	if json.Unmarshal(b, &c) != nil || net.ParseIP(c.Host) == nil || !net.ParseIP(c.Host).IsLoopback() || c.CoreID == "" || c.User == "" || c.Password == "" || c.Database == "" || c.Port < 1 || c.Port > 65535 {
		return nil, errors.New("affectweave: invalid private PG configuration")
	}
	config, err := pgx.ParseConfig("host=127.0.0.1 sslmode=disable")
	if err != nil {
		return nil, err
	}
	config.Host, config.Port, config.User, config.Password, config.Database = c.Host, uint16(c.Port), c.User, c.Password, c.Database
	config.ConnectTimeout = 5 * time.Second
	config.RuntimeParams["application_name"] = "AffectWeave"
	config.RuntimeParams["search_path"] = "aw,pg_catalog"
	config.RuntimeParams["statement_timeout"] = "10000"
	config.RuntimeParams["lock_timeout"] = "3000"
	config.RuntimeParams["idle_in_transaction_session_timeout"] = "15000"
	db := stdlib.OpenDB(*config)
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(time.Minute)
	s := &Store{DB: db, CoreID: c.CoreID}
	if err := s.initialize(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) initialize(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("affectweave: PG unavailable")
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(813001)`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, schema); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO aw.meta(key,value) VALUES('schema','1'),('core',$1) ON CONFLICT DO NOTHING`, s.CoreID); err != nil {
		return err
	}
	var version, core string
	if err = tx.QueryRowContext(ctx, `SELECT value FROM aw.meta WHERE key='schema'`).Scan(&version); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT value FROM aw.meta WHERE key='core'`).Scan(&core); err != nil {
		return err
	}
	if version != "1" || core != s.CoreID {
		return errors.New("affectweave: schema or Core identity mismatch")
	}
	return tx.Commit()
}
func encode(v any) []byte { b, _ := json.Marshal(v); return b }

type Role struct {
	ID       string         `json:"id"`
	Revision int64          `json:"revision"`
	Persona  affect.Persona `json:"persona"`
	Spaces   []string       `json:"spaces"`
}

func (s *Store) Roles(ctx context.Context, space string) ([]Role, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,revision,persona,spaces FROM aw.roles ORDER BY id LIMIT 129`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Role{}
	for rows.Next() {
		var r Role
		var p, b []byte
		if err := rows.Scan(&r.ID, &r.Revision, &p, &b); err != nil {
			return nil, err
		}
		if json.Unmarshal(p, &r.Persona) != nil || json.Unmarshal(b, &r.Spaces) != nil {
			return nil, affect.ErrInvalid
		}
		if space == "" || slices.Contains(r.Spaces, space) {
			result = append(result, r)
		}
	}
	return result, rows.Err()
}
func (s *Store) Read(ctx context.Context, scope affect.Scope) (affect.State, error) {
	if !scope.Valid() || scope.Core != s.CoreID {
		return affect.State{}, ErrScope
	}
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `SELECT s.state FROM aw.states s JOIN aw.roles r ON r.id=s.role_id WHERE s.scope=$1 AND r.spaces ? $2`, scope.Key(), scope.Space).Scan(&raw)
	var state affect.State
	if err == nil {
		err = json.Unmarshal(raw, &state)
	}
	return state, err
}

type Input struct {
	ID    string    `json:"id"`
	RunID string    `json:"run_id"`
	Text  string    `json:"text"`
	At    time.Time `json:"occurred_at"`
}

// Adopt freezes the state for a host-authenticated adopted input. Repeating
// context.prepare reads this exact snapshot, even after later confirmations.
func (s *Store) Adopt(ctx context.Context, scope affect.Scope, in Input) (affect.State, error) {
	if !scope.Valid() || scope.Core != s.CoreID || in.ID == "" || in.RunID == "" || in.At.IsZero() || len(in.Text) > 65536 {
		return affect.State{}, affect.ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return affect.State{}, err
	}
	defer tx.Rollback()
	var p, b []byte
	var revision int64
	if err = tx.QueryRowContext(ctx, `SELECT revision,persona,spaces FROM aw.roles WHERE id=$1 FOR SHARE`, scope.Role).Scan(&revision, &p, &b); err != nil {
		return affect.State{}, err
	}
	var spaces []string
	var persona affect.Persona
	if json.Unmarshal(b, &spaces) != nil || json.Unmarshal(p, &persona) != nil || !slices.Contains(spaces, scope.Space) {
		return affect.State{}, ErrScope
	}
	state := affect.NewState(scope, persona, in.At)
	if _, err = tx.ExecContext(ctx, `INSERT INTO aw.states(scope,core_id,space_id,role_id,actor_id,revision,state) VALUES($1,$2,$3,$4,$5,1,$6) ON CONFLICT DO NOTHING`, scope.Key(), scope.Core, scope.Space, scope.Role, scope.Actor, string(encode(state))); err != nil {
		return affect.State{}, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT state FROM aw.states WHERE scope=$1 FOR UPDATE`, scope.Key()).Scan(&b); err != nil {
		return affect.State{}, err
	}
	if err = json.Unmarshal(b, &state); err != nil {
		return affect.State{}, err
	}
	var oldHash string
	var frozen []byte
	fingerprint := affect.Hash(encode(in))
	err = tx.QueryRowContext(ctx, `SELECT source_hash,snapshot FROM aw.inputs WHERE scope=$1 AND input_id=$2`, scope.Key(), in.ID).Scan(&oldHash, &frozen)
	if err == nil {
		if oldHash != fingerprint {
			return affect.State{}, ErrOperation
		}
		if err = json.Unmarshal(frozen, &state); err != nil {
			return affect.State{}, err
		}
		return state, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return affect.State{}, err
	}
	// A role edit takes effect on the next adopted input, not a repeated hook.
	state.Persona = persona
	affect.Advance(&state, in.At)
	state.Revision++
	if _, err = tx.ExecContext(ctx, `UPDATE aw.states SET revision=$1,state=$2 WHERE scope=$3`, state.Revision, string(encode(state)), scope.Key()); err != nil {
		return affect.State{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO aw.inputs(scope,input_id,source_hash,run_id,snapshot,occurred_at) VALUES($1,$2,$3,$4,$5,$6)`, scope.Key(), in.ID, fingerprint, in.RunID, string(encode(state)), in.At); err != nil {
		return affect.State{}, err
	}
	return state, tx.Commit()
}
func (s *Store) Memories(ctx context.Context, scope affect.Scope) ([]affect.Memory, error) {
	if !scope.Valid() || scope.Core != s.CoreID {
		return nil, ErrScope
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT m.body FROM aw.memories m JOIN aw.states s ON s.scope=m.scope JOIN aw.roles r ON r.id=s.role_id WHERE m.scope=$1 AND r.spaces ? $2 ORDER BY m.created_at DESC,m.id LIMIT 256`, scope.Key(), scope.Space)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []affect.Memory{}
	for rows.Next() {
		var b []byte
		var m affect.Memory
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
func (s *Store) Close() error { return s.DB.Close() }

type Change struct {
	Scope    affect.Scope     `json:"scope"`
	Action   string           `json:"action"`
	Expected int64            `json:"expected_revision"`
	Persona  *affect.Persona  `json:"persona,omitempty"`
	Spaces   []string         `json:"spaces,omitempty"`
	Source   string           `json:"source_id,omitempty"`
	Proposal *affect.Proposal `json:"proposal,omitempty"`
	Decision *affect.Decision `json:"decision,omitempty"`
	Outcome  *Outcome         `json:"outcome,omitempty"`
	MemoryID string           `json:"memory_id,omitempty"`
}
type Outcome struct {
	FeedbackID     string          `json:"feedback_input_id"`
	Evidence       string          `json:"evidence_level"`
	Summary        string          `json:"summary"`
	Proposal       affect.Proposal `json:"proposal"`
	Resolve        []string        `json:"resolved_thread_ids,omitempty"`
	AcceptMemories bool            `json:"accept_memory_candidates"`
}
type Preview struct {
	Revision int64  `json:"revision"`
	Before   any    `json:"before"`
	After    any    `json:"after"`
	Summary  string `json:"summary"`
}
type Result struct {
	Found    bool   `json:"found"`
	ID       string `json:"operation_id"`
	Digest   string `json:"digest"`
	Revision int64  `json:"revision"`
	Action   string `json:"action"`
}

func validateChange(c Change, core string) error {
	if !c.Scope.Valid() || c.Scope.Core != core || c.Expected < 0 {
		return affect.ErrInvalid
	}
	if c.Persona != nil {
		if err := c.Persona.Validate(); err != nil {
			return err
		}
		if c.Persona.ID != c.Scope.Role {
			return affect.ErrInvalid
		}
	}
	if c.Proposal != nil {
		if err := c.Proposal.Validate(); err != nil {
			return err
		}
	}
	if len(c.Spaces) > 128 || len(c.Source) > 256 {
		return affect.ErrInvalid
	}
	for _, v := range c.Spaces {
		if v == "" || len(v) > 256 {
			return affect.ErrInvalid
		}
	}
	switch c.Action {
	case "create_role", "edit_role", "delete_role", "reset_state", "delete_state", "delete_memory", "apply_proposal", "record_outcome":
	default:
		return affect.ErrInvalid
	}
	if (c.Action == "create_role" || c.Action == "edit_role") && (c.Persona == nil || len(c.Spaces) == 0) {
		return affect.ErrInvalid
	}
	if c.Action == "apply_proposal" && (c.Proposal == nil || c.Source == "") {
		return affect.ErrInvalid
	}
	if c.Action == "delete_memory" && (c.MemoryID == "" || len(c.MemoryID) > 256) {
		return affect.ErrInvalid
	}
	if c.Action == "record_outcome" && (c.Outcome == nil || !slices.Contains([]string{"L2", "L3"}, c.Outcome.Evidence) || strings.TrimSpace(c.Outcome.Summary) == "" || c.Source == "") {
		return affect.ErrInvalid
	}
	if c.Outcome != nil {
		if c.Outcome.FeedbackID == "" || len(c.Outcome.Resolve) > 32 {
			return affect.ErrInvalid
		}
		if err := c.Outcome.Proposal.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Preview(ctx context.Context, c Change) (Preview, error) {
	if err := validateChange(c, s.CoreID); err != nil {
		return Preview{}, err
	}
	if strings.HasSuffix(c.Action, "role") {
		var r Role
		var p, b []byte
		err := s.DB.QueryRowContext(ctx, `SELECT id,revision,persona,spaces FROM aw.roles WHERE id=$1`, c.Scope.Role).Scan(&r.ID, &r.Revision, &p, &b)
		if err != nil && !(c.Action == "create_role" && errors.Is(err, sql.ErrNoRows)) {
			return Preview{}, err
		}
		if err == nil {
			_ = json.Unmarshal(p, &r.Persona)
			_ = json.Unmarshal(b, &r.Spaces)
		}
		if r.Revision != c.Expected || c.Action == "create_role" && r.Revision != 0 {
			return Preview{}, ErrConflict
		}
		var after any
		if c.Action != "delete_role" {
			after = Role{ID: c.Scope.Role, Revision: r.Revision + 1, Persona: *c.Persona, Spaces: c.Spaces}
		}
		return Preview{Revision: r.Revision, Before: r, After: after, Summary: c.Action + " " + c.Scope.Role}, nil
	}
	state, err := s.Read(ctx, c.Scope)
	if err != nil {
		return Preview{}, err
	}
	if state.Revision != c.Expected {
		return Preview{}, ErrConflict
	}
	if c.Action == "delete_memory" {
		var body []byte
		if err = s.DB.QueryRowContext(ctx, `SELECT body FROM aw.memories WHERE scope=$1 AND id=$2`, c.Scope.Key(), c.MemoryID).Scan(&body); err != nil {
			return Preview{}, err
		}
		return Preview{Revision: state.Revision, Before: json.RawMessage(body), After: nil, Summary: "delete confirmed memory " + c.MemoryID}, nil
	}
	next, err := nextState(state, c)
	if err != nil {
		return Preview{}, err
	}
	var after any = next
	if c.Action == "delete_state" {
		after = nil
	}
	return Preview{Revision: state.Revision, Before: state, After: after, Summary: c.Action + " for " + c.Scope.Role + " in " + c.Scope.Space}, nil
}
func nextState(state affect.State, c Change) (affect.State, error) {
	next := affect.Clone(state)
	switch c.Action {
	case "reset_state":
		next = affect.NewState(state.Scope, state.Persona, state.Updated)
	case "apply_proposal":
		if err := affect.Apply(&next, *c.Proposal, c.Source, state.Updated, 1); err != nil {
			return next, err
		}
		if c.Decision != nil {
			output := affect.Output{Proposal: *c.Proposal, Decision: *c.Decision, Reply: "validated separately"}
			if err := output.Validate(); err != nil {
				return next, err
			}
			next.Relationship.Stance = output.Decision.Stance
		}
	case "record_outcome":
		m := 1.0
		if c.Outcome.Evidence == "L2" {
			m = .5
		}
		p := c.Outcome.Proposal
		p.Medium = affect.MediumDelta{}
		p.Threads = nil
		if err := affect.Apply(&next, p, c.Source, state.Updated, m); err != nil {
			return next, err
		}
		for _, id := range c.Outcome.Resolve {
			t, ok := next.Threads[id]
			if !ok {
				continue
			}
			if c.Outcome.Evidence == "L3" {
				t.Status = "resolved"
				t.Unresolved = 0
			} else {
				t.Unresolved = affect.Clamp(t.Unresolved-.35, 0, 1)
				if t.Unresolved <= .05 {
					t.Status = "resolved"
				}
			}
			next.Threads[id] = t
		}
	}
	next.Revision = state.Revision + 1
	if len(encode(next)) > 256<<10 {
		return next, affect.ErrInvalid
	}
	return next, nil
}
func (s *Store) Operation(ctx context.Context, id, digest string) (Result, error) {
	var b []byte
	var hash string
	err := s.DB.QueryRowContext(ctx, `SELECT digest,result FROM aw.operations WHERE id=$1`, id).Scan(&hash, &b)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{Found: false, ID: id}, nil
	}
	if err != nil {
		return Result{}, err
	}
	if digest != "" && digest != hash {
		return Result{}, ErrOperation
	}
	var result Result
	err = json.Unmarshal(b, &result)
	return result, err
}

// Apply commits state and the original operation result in one PG transaction.
// There is no automatic replay on a transport failure; the host queries Result.
func (s *Store) Apply(ctx context.Context, id, digest string, c Change) (Result, error) {
	if id == "" || len(id) > 256 || digest == "" || len(digest) > 256 {
		return Result{}, affect.ErrInvalid
	}
	if err := validateChange(c, s.CoreID); err != nil {
		return Result{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	// A lock on this exact operation serializes duplicate requests, including creates.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, id); err != nil {
		return Result{}, err
	}
	fingerprint := affect.Hash(encode(c))
	var previous []byte
	var oldDigest, oldHash string
	err = tx.QueryRowContext(ctx, `SELECT digest,request_hash,result FROM aw.operations WHERE id=$1`, id).Scan(&oldDigest, &oldHash, &previous)
	if err == nil {
		if oldDigest != digest || oldHash != fingerprint {
			return Result{}, ErrOperation
		}
		var result Result
		if err = json.Unmarshal(previous, &result); err != nil {
			return result, err
		}
		return result, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Result{}, err
	}
	result := Result{Found: true, ID: id, Digest: digest, Revision: c.Expected + 1, Action: c.Action}
	if strings.HasSuffix(c.Action, "role") {
		if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,1))`, c.Scope.Role); err != nil {
			return result, err
		}
		var revision int64
		err = tx.QueryRowContext(ctx, `SELECT revision FROM aw.roles WHERE id=$1 FOR UPDATE`, c.Scope.Role).Scan(&revision)
		if errors.Is(err, sql.ErrNoRows) && c.Action == "create_role" {
			err = nil
		}
		if err != nil {
			return result, err
		}
		if revision != c.Expected {
			return result, ErrConflict
		}
		switch c.Action {
		case "create_role":
			if revision != 0 {
				return result, ErrConflict
			}
			var count int
			if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(813002)`); err != nil {
				return result, err
			}
			if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM aw.roles`).Scan(&count); err != nil {
				return result, err
			}
			if count >= 128 {
				return result, errors.New("affectweave: role limit reached")
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO aw.roles(id,revision,persona,spaces,updated_at) VALUES($1,1,$2,$3,now())`, c.Scope.Role, string(encode(c.Persona)), string(encode(c.Spaces)))
		case "edit_role":
			_, err = tx.ExecContext(ctx, `UPDATE aw.roles SET revision=revision+1,persona=$1,spaces=$2,updated_at=now() WHERE id=$3`, string(encode(c.Persona)), string(encode(c.Spaces)), c.Scope.Role)
		case "delete_role":
			if _, err = tx.ExecContext(ctx, `DELETE FROM aw.states WHERE role_id=$1`, c.Scope.Role); err == nil {
				_, err = tx.ExecContext(ctx, `DELETE FROM aw.roles WHERE id=$1`, c.Scope.Role)
			}
		}
	} else {
		var allowed []byte
		if err = tx.QueryRowContext(ctx, `SELECT spaces FROM aw.roles WHERE id=$1 FOR SHARE`, c.Scope.Role).Scan(&allowed); err != nil {
			return result, err
		}
		var spaces []string
		if json.Unmarshal(allowed, &spaces) != nil || !slices.Contains(spaces, c.Scope.Space) {
			return result, ErrScope
		}
		var raw []byte
		if err = tx.QueryRowContext(ctx, `SELECT state FROM aw.states WHERE scope=$1 FOR UPDATE`, c.Scope.Key()).Scan(&raw); err != nil {
			return result, err
		}
		var state affect.State
		if err = json.Unmarshal(raw, &state); err != nil {
			return result, err
		}
		if state.Revision != c.Expected {
			return result, ErrConflict
		}
		if c.Source != "" {
			var present bool
			if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM aw.inputs WHERE scope=$1 AND input_id=$2 UNION ALL SELECT 1 FROM aw.facts WHERE scope=$1 AND id=$2)`, c.Scope.Key(), c.Source).Scan(&present); err != nil || !present {
				return result, errors.New("affectweave: source fact missing")
			}
		}
		if c.Action == "record_outcome" {
			var kind string
			var body []byte
			if err = tx.QueryRowContext(ctx, `SELECT kind,body FROM aw.facts WHERE scope=$1 AND id=$2`, c.Scope.Key(), c.Source).Scan(&kind, &body); err != nil {
				return result, errors.New("affectweave: delivered source required")
			}
			var receipt struct {
				State string `json:"state"`
			}
			if json.Unmarshal(body, &receipt) != nil || kind != "delivery" || receipt.State != "sent" && receipt.State != "partial" {
				return result, errors.New("affectweave: unknown/failed generation is not delivered evidence")
			}
			var feedback bool
			if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM aw.inputs WHERE scope=$1 AND input_id=$2)`, c.Scope.Key(), c.Outcome.FeedbackID).Scan(&feedback); err != nil || !feedback {
				return result, errors.New("affectweave: actual adopted feedback input required")
			}
		}
		next, buildErr := nextState(state, c)
		if buildErr != nil {
			return result, buildErr
		}
		if c.Action == "delete_state" {
			_, err = tx.ExecContext(ctx, `DELETE FROM aw.states WHERE scope=$1`, c.Scope.Key())
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE aw.states SET revision=$1,state=$2 WHERE scope=$3`, next.Revision, string(encode(next)), c.Scope.Key())
		}
		if err != nil {
			return result, err
		}
		if c.Action == "reset_state" {
			if _, err = tx.ExecContext(ctx, `DELETE FROM aw.memories WHERE scope=$1`, c.Scope.Key()); err != nil {
				return result, err
			}
		}
		if c.Action == "record_outcome" && c.Outcome.AcceptMemories {
			for n, m := range c.Outcome.Proposal.Memories {
				if m.Salience < .20 {
					continue
				}
				m.ID = affect.Hash([]byte(fmt.Sprintf("%s:%d", id, n)))
				m.Salience = affect.Clamp(m.Salience, 0, 1)
				m.Confidence = affect.Clamp(m.Confidence, 0, 1)
				m.Sources = []string{c.Source}
				m.Evidence = c.Outcome.Evidence
				m.Created = state.Updated
				m.Entities = append(m.Entities, c.Scope.Actor)
				if _, err = tx.ExecContext(ctx, `INSERT INTO aw.memories(scope,id,body,created_at) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, c.Scope.Key(), m.ID, string(encode(m)), m.Created); err != nil {
					return result, err
				}
			}
		}
	}
	if err != nil {
		return result, err
	}
	if c.Action == "delete_memory" {
		r, e := tx.ExecContext(ctx, `DELETE FROM aw.memories WHERE scope=$1 AND id=$2`, c.Scope.Key(), c.MemoryID)
		if e != nil {
			return result, e
		}
		n, _ := r.RowsAffected()
		if n != 1 {
			return result, errors.New("affectweave: memory no longer exists")
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO aw.operations(id,digest,request_hash,request,result,applied_at) VALUES($1,$2,$3,$4,$5,now())`, id, digest, fingerprint, string(encode(c)), string(encode(result))); err != nil {
		return result, err
	}
	return result, tx.Commit()
}

type Fact struct {
	ID   string          `json:"id"`
	Kind string          `json:"kind"`
	Body json.RawMessage `json:"body"`
	At   time.Time       `json:"occurred_at"`
}

func (s *Store) RecordFact(ctx context.Context, scope affect.Scope, f Fact) error {
	if !scope.Valid() || scope.Core != s.CoreID || f.ID == "" || len(f.ID) > 256 || len(f.Body) > 131072 || !json.Valid(f.Body) || f.At.IsZero() {
		return affect.ErrInvalid
	}
	switch f.Kind {
	case "generated", "delivery", "outcome":
	default:
		return affect.ErrInvalid
	}
	r, err := s.DB.ExecContext(ctx, `INSERT INTO aw.facts(scope,id,kind,body,occurred_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, scope.Key(), f.ID, f.Kind, string(f.Body), f.At)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 1 {
		return nil
	}
	var kind string
	var b []byte
	if err = s.DB.QueryRowContext(ctx, `SELECT kind,body FROM aw.facts WHERE scope=$1 AND id=$2`, scope.Key(), f.ID).Scan(&kind, &b); err != nil {
		return err
	}
	var old, new any
	_ = json.Unmarshal(b, &old)
	_ = json.Unmarshal(f.Body, &new)
	if kind != f.Kind || string(encode(old)) != string(encode(new)) {
		return ErrOperation
	}
	return nil
}

func (s *Store) Facts(ctx context.Context, scope affect.Scope, limit int) ([]Fact, error) {
	if !scope.Valid() || scope.Core != s.CoreID || limit < 1 || limit > 100 {
		return nil, ErrScope
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT f.id,f.kind,f.body,f.occurred_at FROM aw.facts f JOIN aw.states s ON s.scope=f.scope JOIN aw.roles r ON r.id=s.role_id WHERE f.scope=$1 AND r.spaces ? $3 ORDER BY f.occurred_at DESC,f.id LIMIT $2`, scope.Key(), limit, scope.Space)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Fact{}
	for rows.Next() {
		var f Fact
		if err := rows.Scan(&f.ID, &f.Kind, &f.Body, &f.At); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, rows.Err()
}
