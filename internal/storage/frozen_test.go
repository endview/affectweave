package storage_test

import (
	"github.com/endview/affectweave/internal/affect"
	"github.com/endview/affectweave/internal/storage"
	"github.com/endview/affectweave/internal/testpg"
	"testing"
	"time"
)

func TestRealPGCompleteFragmentFrozenAcrossMemoryAndSettingsChanges(t *testing.T) {
	s, path := testpg.Open(t)
	scope, in := seed(t, s)
	a, err := s.AdoptFrozen(t.Context(), scope, in, storage.InputOptions{})
	if err != nil {
		t.Fatal(err)
	}
	memory := affect.Memory{ID: "later", Summary: "Later confirmed fact", Salience: 1, Confidence: 1, Created: in.At}
	if _, err = s.DB.ExecContext(t.Context(), `INSERT INTO aw.memories(scope,id,body,created_at) VALUES($1,$2,$3,$4)`, scope.Key(), memory.ID, `{"id":"later","summary":"Later confirmed fact","salience":1,"confidence":1}`, in.At); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	b, err := reopened.AdoptFrozen(t.Context(), scope, in, storage.InputOptions{MaxBytes: 12000, Analysis: true})
	if err != nil || a.Context.Text != b.Context.Text || b.Options.Analysis {
		t.Fatalf("fragment/settings not frozen: %v", err)
	}
	in.ID = "new"
	in.RunID = "new-run"
	in.At = in.At.Add(time.Minute)
	c, err := s.AdoptFrozen(t.Context(), scope, in, storage.InputOptions{Analysis: true})
	if err != nil || len(c.Memories) != 1 || c.Analysis != "pending" {
		t.Fatal(c, err)
	}
}
