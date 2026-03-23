package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// API provides the REST + SSE endpoints for the Visual Editor.
type API struct {
	store *EventStore
	mux   *http.ServeMux
}

// NewAPI creates a new API handler wired to the given store.
func NewAPI(store *EventStore) *API {
	a := &API{store: store, mux: http.NewServeMux()}
	a.registerRoutes()
	return a
}

// Handler returns the HTTP handler for the API.
func (a *API) Handler() http.Handler { return a.mux }

func (a *API) registerRoutes() {
	a.mux.HandleFunc("GET /api/circuits", a.handleListCircuits)
	a.mux.HandleFunc("GET /api/runs", a.handleListRuns)
	a.mux.HandleFunc("GET /api/runs/{runID}", a.handleGetRun)
	a.mux.HandleFunc("GET /api/runs/{runID}/events", a.handleRunEvents)
	a.mux.HandleFunc("GET /api/runs/{runID}/events/stream", a.handleRunSSE)
}

func (a *API) handleListCircuits(w http.ResponseWriter, _ *http.Request) {
	runs := a.store.Runs()
	circuits := map[string]bool{}
	for _, r := range runs {
		circuits[r.Circuit] = true
	}
	names := make([]string, 0, len(circuits))
	for name := range circuits {
		names = append(names, name)
	}
	writeJSON(w, http.StatusOK, map[string]any{"circuits": names})
}

func (a *API) handleListRuns(w http.ResponseWriter, _ *http.Request) {
	runs := a.store.Runs()
	writeJSON(w, http.StatusOK, runs)
}

func (a *API) handleGetRun(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("runID")
	run := a.store.Run(runID)
	if run == nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (a *API) handleRunEvents(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("runID")
	afterStr := r.URL.Query().Get("after")
	afterID := 0
	if afterStr != "" {
		afterID, _ = strconv.Atoi(afterStr)
	}

	events := a.store.EventsSince(runID, afterID)
	writeJSON(w, http.StatusOK, events)
}

func (a *API) handleRunSSE(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("runID")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher.Flush()

	subID, ch := a.store.Subscribe()
	defer a.store.Unsubscribe(subID)

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			if !strings.EqualFold(evt.RunID, runID) {
				continue
			}
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
