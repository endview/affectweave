package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/endview/affectweave/internal/affect"
	"github.com/endview/affectweave/internal/storage"
	"github.com/endview/affectweave/internal/testpg"
	"testing"
	"time"
)

func TestProtocolHandshakeAndBoundedFrames(t *testing.T) {
	var output bytes.Buffer
	s := &Server{}
	input := "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocol\":\"freeagent-ext/1\",\"state_version\":2,\"capabilities\":[\"context_analysis\",\"scheduled_generation\"]}}\n"
	if err := s.Serve(t.Context(), bytes.NewBufferString(input), &output); err != nil {
		t.Fatal(err)
	}
	var reply struct {
		Result struct {
			ID       string   `json:"id"`
			Provides []string `json:"provides"`
		} `json:"result"`
	}
	if json.Unmarshal(bytes.TrimSpace(output.Bytes()), &reply) != nil || reply.Result.ID != ID || len(reply.Result.Provides) != 3 {
		t.Fatal(output.String())
	}
	if err := s.Serve(t.Context(), bytes.NewBuffer(bytes.Repeat([]byte("x"), 1<<20+1)), &bytes.Buffer{}); err == nil {
		t.Fatal("oversized frame accepted")
	}
	if _, err := s.handle(t.Context(), "initialize", []byte(`{"protocol":"freeagent-ext/1","state_version":2,"capabilities":["context_analysis"]}`)); err == nil {
		t.Fatal("missing required host capability accepted")
	}
}
func TestPauseCancellationReleasesAndDeadlineIsBounded(t *testing.T) {
	s := &Server{writes: 1}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := s.pause(ctx, "pause", time.Now().Add(time.Minute)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	s.writes = 0
	if err := s.beginWrite(); err != nil {
		t.Fatal("cancel left writeback paused", err)
	}
	s.endWrite()
	if err := s.pause(t.Context(), "pause", time.Now().Add(3*time.Minute)); err == nil {
		t.Fatal("unbounded pause accepted")
	}
}
func TestStrictStateRequestRejectsProtectedFields(t *testing.T) {
	var change struct {
		ID string `json:"id"`
		storage.Change
	}
	for _, raw := range []string{`{"action":"apply_proposal","proposal":{"permissions":"full"}}`, `{"action":"apply_proposal","proposal":{"relationship_delta":{"tools":[]}}}`, `{"action":"create_role","persona":{"id":"a","system_prompt":"replace"}}`} {
		if strictJSON([]byte(raw), &change) == nil {
			t.Fatal("protected/unknown field accepted", raw)
		}
	}
}
func TestRealPGAdapterPreservesGenerationAndDeliveryLevels(t *testing.T) {
	store, _ := testpg.Open(t)
	s := &Server{Store: store}
	scope := affect.Scope{Core: store.CoreID, Space: "local", Role: "maya", Actor: "owner"}
	p := affect.Persona{ID: "maya", Name: "Maya", Identity: "Engineer", Version: 1, Frozen: true}
	if _, err := store.Apply(t.Context(), "role", "digest", storage.Change{Scope: scope, Action: "create_role", Persona: &p, Spaces: []string{"local"}}); err != nil {
		t.Fatal(err)
	}
	params := Params{Identity: &Identity{Core: store.CoreID, Space: "local", Conversation: "chat", Run: "run", Inputs: []Adopted{{ID: "chat/1", Actor: "owner", Text: "hello", At: time.Now().UTC()}}}, Variant: "maya", Output: "generated text", Status: "unknown"}
	b, _ := json.Marshal(params)
	if _, err := s.handle(t.Context(), "context.prepare", b); err != nil {
		t.Fatal(err)
	}
	if _, err := s.handle(t.Context(), "turn.finished", b); err != nil {
		t.Fatal(err)
	}
	facts, err := store.Facts(t.Context(), scope, 50)
	var generated struct {
		Delivery string `json:"delivery"`
		Evidence string `json:"evidence_level"`
	}
	if len(facts) == 1 {
		_ = json.Unmarshal(facts[0].Body, &generated)
	}
	if err != nil || len(facts) != 1 || generated.Delivery != "not_observed" || generated.Evidence != "L0" {
		t.Fatal(facts, err)
	}
	for _, state := range []string{"sent", "partial", "unknown", "failed"} {
		body, _ := json.Marshal(map[string]string{"receipt_id": "receipt:" + state, "state": state})
		if err := store.RecordFact(t.Context(), scope, storage.Fact{ID: "receipt:" + state, Kind: "delivery", Body: body, At: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	facts, err = store.Facts(t.Context(), scope, 50)
	if err != nil || len(facts) != 5 {
		t.Fatal(facts, err)
	}
	current, _ := store.Read(t.Context(), scope)
	if current.Relationship.Trust != .5 || current.Revision != 2 {
		t.Fatal("delivery implicitly learned", current)
	}
}

func TestRealPGFactSyncDoesNotReprepareAnExistingInput(t *testing.T) {
	store, _ := testpg.Open(t)
	s := &Server{Store: store}
	scope := affect.Scope{Core: store.CoreID, Space: "local", Role: "maya", Actor: "owner"}
	persona := affect.Persona{ID: "maya", Name: "Maya", Identity: "Synthetic engineering partner", Version: 1, Frozen: true}
	if _, err := store.Apply(t.Context(), "role", "digest", storage.Change{Scope: scope, Action: "create_role", Persona: &persona, Spaces: []string{"local"}}); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	input := storage.Input{ID: "chat/1", RunID: "old-run", Text: "Original source", At: at}
	state, err := store.Adopt(t.Context(), scope, input)
	if err != nil {
		t.Fatal(err)
	}
	// A fact reread has no authority to regenerate a missing immutable fragment.
	if _, err = store.DB.ExecContext(t.Context(), `UPDATE aw.inputs SET context_snapshot='{}' WHERE scope=$1 AND input_id=$2`, scope.Key(), input.ID); err != nil {
		t.Fatal(err)
	}
	f := RunFacts{Identity: &Identity{Core: store.CoreID, Space: scope.Space, Conversation: "chat", Run: input.RunID, Inputs: []Adopted{{ID: input.ID, Actor: scope.Actor, Text: input.Text, At: at}}}, Status: "completed", Output: "Original answer", Updated: at}
	if err = s.syncFacts(t.Context(), scope.Role, f); err != nil {
		t.Fatal("fact reread tried to reconstruct the input context", err)
	}
	current, err := store.Read(t.Context(), scope)
	if err != nil || current.Revision != state.Revision {
		t.Fatal("fact reread advanced state", current.Revision, err)
	}
	var raw string
	if err = store.DB.QueryRowContext(t.Context(), `SELECT context_snapshot::text FROM aw.inputs WHERE scope=$1 AND input_id=$2`, scope.Key(), input.ID).Scan(&raw); err != nil || raw != "{}" {
		t.Fatal("fact reread rewrote frozen fragment", raw, err)
	}
	if _, err = store.Adopt(t.Context(), scope, input); err == nil {
		t.Fatal("ordinary repeated hook accepted an incomplete fragment")
	}
	f.Identity.Inputs[0].Text = "Changed historical source"
	if err = s.syncFacts(t.Context(), scope.Role, f); !errors.Is(err, storage.ErrOperation) {
		t.Fatal("changed source was accepted", err)
	}
	if _, err = store.Adopt(t.Context(), scope, storage.Input{ID: "chat/2", RunID: "new-run", Text: "New input", At: at.Add(time.Second)}); err != nil {
		t.Fatal("new input could not prepare", err)
	}
}
