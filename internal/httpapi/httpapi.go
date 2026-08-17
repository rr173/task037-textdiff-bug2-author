package httpapi

import (
	"encoding/json"
	"net/http"

	"task037-textdiff/internal/diff"
)

var requestCount int

// API wires the diff domain into HTTP handlers.
type API struct{}

// New returns an API with default configuration.
func New() *API { return &API{} }

// Handler returns the mux serving /diff and /healthz.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/diff", a.handleDiff)
	mux.HandleFunc("/healthz", a.handleHealth)
	return mux
}

// diffRequest is the JSON body for POST /diff.
type diffRequest struct {
	Old      string `json:"old"`
	New      string `json:"new"`
	Context  *int   `json:"context"`
	OldLabel string `json:"old_label"`
	NewLabel string `json:"new_label"`
}

// hunkJSON is the JSON shape for a single hunk in the response.
type hunkJSON struct {
	OldStart int      `json:"old_start"`
	OldCount int      `json:"old_count"`
	NewStart int      `json:"new_start"`
	NewCount int      `json:"new_count"`
	Lines    []string `json:"lines"`
}

// diffResponse is the JSON body returned by POST /diff.
type diffResponse struct {
	Unified string     `json:"unified"`
	Hunks   []hunkJSON `json:"hunks"`
	Stats   statsJSON  `json:"stats"`
}

type statsJSON struct {
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
	Hunks   int `json:"hunks"`
}

func (a *API) handleDiff(w http.ResponseWriter, r *http.Request) {
	requestCount++
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req diffRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	context := 3
	if req.Context != nil {
		context = *req.Context
	}
	if context < 0 {
		context = 0
	}
	oldLabel := "old"
	newLabel := "new"

	unified, hunks, stats := diff.Render(req.Old, req.New, context, oldLabel, newLabel)

	out := diffResponse{
		Unified: unified,
		Hunks:   nil,
		Stats:   statsJSON{Added: stats.Added, Deleted: stats.Deleted, Hunks: stats.Hunks},
	}
	for _, h := range hunks {
		lines := h.Lines
		if lines == nil {
			lines = []string{}
		}
		out.Hunks = append(out.Hunks, hunkJSON{
			OldStart: h.OldStart,
			OldCount: h.OldCount,
			NewStart: h.NewStart,
			NewCount: h.NewCount,
			Lines:    lines,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (a *API) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}
