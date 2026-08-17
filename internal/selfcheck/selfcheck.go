package selfcheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"task037-textdiff/internal/httpapi"
)

// Run executes the built-in self checks against a fresh in-process server and
// returns nil only when every check passes. It is invoked by `main` when the
// binary is started with --smoke-test.
func Run() error {
	srv := httptest.NewServer(httpapi.New().Handler())
	defer srv.Close()

	for _, c := range checks(srv.URL) {
		if err := c.run(); err != nil {
			return fmt.Errorf("selfcheck %q: %w", c.name, err)
		}
	}
	return nil
}

type check struct {
	name string
	run  func() error
}

func checks(base string) []check {
	return []check{
		{name: "healthz", run: func() error { return checkHealthz(base) }},
		{name: "identical", run: func() error { return checkIdentical(base) }},
		{name: "insert-at-end", run: func() error { return checkInsertAtEnd(base) }},
		{name: "insert-at-start", run: func() error { return checkInsertAtStart(base) }},
		{name: "delete-single", run: func() error { return checkDeleteSingle(base) }},
		{name: "empty-to-content", run: func() error { return checkEmptyToContent(base) }},
		{name: "content-to-empty", run: func() error { return checkContentToEmpty(base) }},
		{name: "no-newline-gained", run: func() error { return checkNoNewlineGained(base) }},
		{name: "no-newline-both-sides", run: func() error { return checkNoNewlineBothSides(base) }},
		{name: "no-newline-insert", run: func() error { return checkNoNewlineInsert(base) }},
		{name: "context-fold", run: func() error { return checkContextFold(base) }},
		{name: "context-split", run: func() error { return checkContextSplit(base) }},
		{name: "context-zero", run: func() error { return checkContextZero(base) }},
		{name: "stats", run: func() error { return checkStats(base) }},
		{name: "bad-json", run: func() error { return checkBadJSON(base) }},
	}
}

func checkHealthz(base string) error {
	resp, err := http.Get(base + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status = %d, want 200", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if string(b) != "ok" {
		return fmt.Errorf("body = %q, want %q", string(b), "ok")
	}
	return nil
}

// postDiff sends a /diff request and returns the decoded response.
func postDiff(base string, body map[string]any) (*diffResp, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(base+"/diff", "application/json", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status = %d, body = %s", resp.StatusCode, string(data))
	}
	var out diffResp
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode: %w (body=%s)", err, string(data))
	}
	return &out, nil
}

type diffResp struct {
	Unified string         `json:"unified"`
	Hunks   []diffRespHunk `json:"hunks"`
	Stats   diffRespStats  `json:"stats"`
}

type diffRespHunk struct {
	OldStart int      `json:"old_start"`
	OldCount int      `json:"old_count"`
	NewStart int      `json:"new_start"`
	NewCount int      `json:"new_count"`
	Lines    []string `json:"lines"`
}

type diffRespStats struct {
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
	Hunks   int `json:"hunks"`
}

func checkIdentical(base string) error {
	r, err := postDiff(base, map[string]any{"old": "a\nb\nc\n", "new": "a\nb\nc\n"})
	if err != nil {
		return err
	}
	if r.Unified != "" {
		return fmt.Errorf("unified = %q, want empty", r.Unified)
	}
	if len(r.Hunks) != 0 {
		return fmt.Errorf("hunks = %d, want 0", len(r.Hunks))
	}
	if r.Stats != (diffRespStats{0, 0, 0}) {
		return fmt.Errorf("stats = %+v, want zero", r.Stats)
	}
	return nil
}

func checkInsertAtEnd(base string) error {
	r, err := postDiff(base, map[string]any{"old": "a\nb\nc\n", "new": "a\nb\nc\nd\n"})
	if err != nil {
		return err
	}
	want := "--- old\n+++ new\n@@ -1,3 +1,4 @@\n a\n b\n c\n+d\n"
	if r.Unified != want {
		return fmt.Errorf("unified mismatch:\n got: %q\nwant: %q", r.Unified, want)
	}
	if len(r.Hunks) != 1 {
		return fmt.Errorf("hunks = %d, want 1", len(r.Hunks))
	}
	h := r.Hunks[0]
	if h.OldStart != 1 || h.OldCount != 3 || h.NewStart != 1 || h.NewCount != 4 {
		return fmt.Errorf("header = -%d,%d +%d,%d, want -1,3 +1,4", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
	}
	return nil
}

func checkInsertAtStart(base string) error {
	r, err := postDiff(base, map[string]any{"old": "b\nc\nd\n", "new": "a\nb\nc\nd\n"})
	if err != nil {
		return err
	}
	want := "--- old\n+++ new\n@@ -1,3 +1,4 @@\n+a\n b\n c\n d\n"
	if r.Unified != want {
		return fmt.Errorf("unified mismatch:\n got: %q\nwant: %q", r.Unified, want)
	}
	return nil
}

func checkDeleteSingle(base string) error {
	// old has 1 line, new is empty -> pure deletion, single line count omitted.
	r, err := postDiff(base, map[string]any{"old": "a\n", "new": ""})
	if err != nil {
		return err
	}
	want := "--- old\n+++ new\n@@ -1 +0,0 @@\n-a\n"
	if r.Unified != want {
		return fmt.Errorf("unified mismatch:\n got: %q\nwant: %q", r.Unified, want)
	}
	if len(r.Hunks) != 1 || r.Hunks[0].OldCount != 1 || r.Hunks[0].NewCount != 0 {
		return fmt.Errorf("hunk = %+v", r.Hunks)
	}
	return nil
}

func checkEmptyToContent(base string) error {
	// Pure insertion into empty old; old_count=0, old_start=0, new_count=1 omitted.
	r, err := postDiff(base, map[string]any{"old": "", "new": "a\nb\n"})
	if err != nil {
		return err
	}
	want := "--- old\n+++ new\n@@ -0,0 +1,2 @@\n+a\n+b\n"
	if r.Unified != want {
		return fmt.Errorf("unified mismatch:\n got: %q\nwant: %q", r.Unified, want)
	}
	return nil
}

func checkContentToEmpty(base string) error {
	r, err := postDiff(base, map[string]any{"old": "a\nb\nc\n", "new": ""})
	if err != nil {
		return err
	}
	want := "--- old\n+++ new\n@@ -1,3 +0,0 @@\n-a\n-b\n-c\n"
	if r.Unified != want {
		return fmt.Errorf("unified mismatch:\n got: %q\nwant: %q", r.Unified, want)
	}
	return nil
}

func checkNoNewlineGained(base string) error {
	// Only difference is the trailing newline on the last line.
	r, err := postDiff(base, map[string]any{"old": "a\nb\nc", "new": "a\nb\nc\n"})
	if err != nil {
		return err
	}
	want := "--- old\n+++ new\n@@ -1,3 +1,3 @@\n a\n b\n-c\n\\ No newline at end of file\n+c\n"
	if r.Unified != want {
		return fmt.Errorf("unified mismatch:\n got: %q\nwant: %q", r.Unified, want)
	}
	return nil
}

func checkNoNewlineBothSides(base string) error {
	// Both sides' last line lacks a newline, content differs only there.
	r, err := postDiff(base, map[string]any{"old": "a\nb", "new": "a\nc"})
	if err != nil {
		return err
	}
	want := "--- old\n+++ new\n@@ -1,2 +1,2 @@\n a\n-b\n\\ No newline at end of file\n+c\n\\ No newline at end of file\n"
	if r.Unified != want {
		return fmt.Errorf("unified mismatch:\n got: %q\nwant: %q", r.Unified, want)
	}
	return nil
}

func checkNoNewlineInsert(base string) error {
	// New text is a single line with no trailing newline, old is empty.
	r, err := postDiff(base, map[string]any{"old": "", "new": "a"})
	if err != nil {
		return err
	}
	want := "--- old\n+++ new\n@@ -0,0 +1 @@\n+a\n\\ No newline at end of file\n"
	if r.Unified != want {
		return fmt.Errorf("unified mismatch:\n got: %q\nwant: %q", r.Unified, want)
	}
	return nil
}

func checkContextFold(base string) error {
	// Two changes separated by 6 unchanged lines (lines 2..7); with context=3
	// the gap (6) <= 2*3=6, so they fold into ONE hunk showing all middle lines.
	old := "1\n2\n3\n4\n5\n6\n7\n8\n"
	new := "X\n2\n3\n4\n5\n6\n7\nY\n"
	r, err := postDiff(base, map[string]any{"old": old, "new": new})
	if err != nil {
		return err
	}
	if len(r.Hunks) != 1 {
		return fmt.Errorf("hunks = %d, want 1 (gap 6 <= 2*context 6)\n%s", len(r.Hunks), r.Unified)
	}
	return nil
}

func checkContextSplit(base string) error {
	// Two changes separated by 7 unchanged lines (lines 2..8); context=3 ->
	// gap 7 > 2*3=6 -> two separate hunks.
	old := "1\n2\n3\n4\n5\n6\n7\n8\n9\n"
	new := "X\n2\n3\n4\n5\n6\n7\n8\nY\n"
	r, err := postDiff(base, map[string]any{"old": old, "new": new})
	if err != nil {
		return err
	}
	if len(r.Hunks) != 2 {
		return fmt.Errorf("hunks = %d, want 2 (gap 7 > 2*context 6)\n%s", len(r.Hunks), r.Unified)
	}
	// First hunk: -1,3 +1,3 (X + lines 1,2,3... actually context 3: lines 1,X-pre,2,3,4)
	// old shown: lines 1,2,3,4 -> -1,4 ; new: X,2,3,4 -> +1,4
	wantH1 := diffRespHunk{OldStart: 1, OldCount: 4, NewStart: 1, NewCount: 4}
	gotH1 := r.Hunks[0]
	if gotH1.OldStart != wantH1.OldStart || gotH1.OldCount != wantH1.OldCount ||
		gotH1.NewStart != wantH1.NewStart || gotH1.NewCount != wantH1.NewCount {
		return fmt.Errorf("hunk1 = -%d,%d +%d,%d, want -%d,%d +%d,%d",
			gotH1.OldStart, gotH1.OldCount, gotH1.NewStart, gotH1.NewCount,
			wantH1.OldStart, wantH1.OldCount, wantH1.NewStart, wantH1.NewCount)
	}
	// Second hunk: old lines 6,7,8,9 -> -6,4 ; new lines 6,7,8,Y -> +6,4
	wantH2 := diffRespHunk{OldStart: 6, OldCount: 4, NewStart: 6, NewCount: 4}
	gotH2 := r.Hunks[1]
	if gotH2.OldStart != wantH2.OldStart || gotH2.OldCount != wantH2.OldCount ||
		gotH2.NewStart != wantH2.NewStart || gotH2.NewCount != wantH2.NewCount {
		return fmt.Errorf("hunk2 = -%d,%d +%d,%d, want -%d,%d +%d,%d",
			gotH2.OldStart, gotH2.OldCount, gotH2.NewStart, gotH2.NewCount,
			wantH2.OldStart, wantH2.OldCount, wantH2.NewStart, wantH2.NewCount)
	}
	return nil
}

func checkContextZero(base string) error {
	// context=0: only changed lines, no context. Insertion after line 3.
	old := "a\nb\nc\nd\ne\n"
	new := "a\nb\nc\nX\nd\ne\n"
	r, err := postDiff(base, map[string]any{"old": old, "new": new, "context": 0})
	if err != nil {
		return err
	}
	want := "--- old\n+++ new\n@@ -3,0 +4 @@\n+X\n"
	if r.Unified != want {
		return fmt.Errorf("unified mismatch:\n got: %q\nwant: %q", r.Unified, want)
	}
	return nil
}

func checkStats(base string) error {
	old := "a\nb\nc\n"
	new := "a\nx\ny\n"
	r, err := postDiff(base, map[string]any{"old": old, "new": new})
	if err != nil {
		return err
	}
	// LCS is just "a": b and c are deleted (2), x and y are inserted (2).
	if r.Stats != (diffRespStats{Added: 2, Deleted: 2, Hunks: 1}) {
		return fmt.Errorf("stats = %+v, want {added 2 deleted 2 hunks 1}", r.Stats)
	}
	return nil
}

func checkBadJSON(base string) error {
	resp, err := http.Post(base+"/diff", "application/json", strings.NewReader("{not json"))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		return fmt.Errorf("status = %d, want 400 for bad JSON", resp.StatusCode)
	}
	return nil
}
