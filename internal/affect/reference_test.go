package affect

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestFixedPythonReferenceScenarios(t *testing.T) {
	b, err := os.ReadFile("testdata/python-reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Commit string `json:"reference_commit"`
		Cases  []struct {
			Name      string     `json:"name"`
			Proposals []Proposal `json:"proposals"`
			Advance   int        `json:"advance_seconds"`
			Expected  struct {
				Medium       Medium             `json:"medium"`
				Relationship Relationship       `json:"relationship"`
				Emotions     map[string]float64 `json:"emotions"`
				Threads      map[string]struct {
					Unresolved float64 `json:"unresolvedness"`
					Status     string  `json:"status"`
				} `json:"threads"`
			} `json:"expected"`
		} `json:"cases"`
	}
	if json.Unmarshal(b, &fixture) != nil || fixture.Commit != "d8be9d3255006c51a96c114e36949771ae26517c" || len(fixture.Cases) != 5 {
		t.Fatal("invalid fixed reference fixture")
	}
	for _, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			s := base()
			for n, p := range c.Proposals {
				if err := Apply(&s, p, fmt.Sprint("event-", n), s.Updated, 1); err != nil {
					t.Fatal(err)
				}
			}
			if c.Advance > 0 {
				Advance(&s, s.Updated.Add(time.Duration(c.Advance)*time.Second))
			}
			got, want := s.Medium, c.Expected.Medium
			closeTo(t, got.Valence, want.Valence)
			closeTo(t, got.Activation, want.Activation)
			closeTo(t, got.Energy, want.Energy)
			closeTo(t, got.Stress, want.Stress)
			if !reflect.DeepEqual(s.Relationship, c.Expected.Relationship) {
				t.Fatal(s.Relationship, c.Expected.Relationship)
			}
			if len(s.Emotions) != len(c.Expected.Emotions) {
				t.Fatal("emotion count")
			}
			for _, e := range s.Emotions {
				closeTo(t, e.Intensity, c.Expected.Emotions[e.CauseID+":"+e.Type])
			}
			if len(s.Threads) != len(c.Expected.Threads) {
				t.Fatal("thread count")
			}
			for id, thread := range s.Threads {
				want := c.Expected.Threads[id]
				closeTo(t, thread.Unresolved, want.Unresolved)
				if thread.Status != want.Status {
					t.Fatal(thread, want)
				}
			}
		})
	}
}
