package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

// Job is one long-running background operation shown in the UI.
type Job struct {
	ID       string      `json:"id"`
	Kind     string      `json:"kind"`
	Label    string      `json:"label"`
	Stage    string      `json:"stage"`
	State    string      `json:"state"`
	Done     int64       `json:"done"`
	Total    int64       `json:"total"`
	Error    string      `json:"error,omitempty"`
	Result   interface{} `json:"result,omitempty"`
	Started  time.Time   `json:"started"`
	Finished time.Time   `json:"finished,omitempty"`
}

type jobState struct {
	job    Job
	cancel context.CancelFunc
}

// JobManager tracks background downloads and setup jobs.
type JobManager struct {
	mu   sync.RWMutex
	jobs map[string]*jobState
	all  []string
}

// NewJobs creates an empty job manager.
func NewJobs() *JobManager {
	return &JobManager{jobs: map[string]*jobState{}}
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().Format("150405.000000")
	}
	return hex.EncodeToString(b[:])
}

// Start runs fn in the background and exposes its progress as a Job.
func (m *JobManager) Start(kind, label string, fn func(context.Context, *Job) (interface{}, error)) *Job {
	ctx, cancel := context.WithCancel(context.Background())
	job := Job{
		ID: newID(), Kind: kind, Label: label, Stage: "starting",
		State: "running", Started: time.Now().UTC(),
	}
	st := &jobState{job: job, cancel: cancel}
	m.mu.Lock()
	m.jobs[job.ID] = st
	m.all = append(m.all, job.ID)
	m.mu.Unlock()

	go func() {
		result, err := fn(ctx, &job)
		m.mu.Lock()
		defer m.mu.Unlock()
		st := m.jobs[job.ID]
		if st == nil {
			return
		}
		st.job.Result = result
		if err != nil {
			st.job.State = "error"
			st.job.Error = err.Error()
			st.job.Stage = "failed"
		} else {
			st.job.State = "done"
			if st.job.Stage == "starting" || st.job.Stage == "" {
				st.job.Stage = "done"
			}
		}
		st.job.Finished = time.Now().UTC()
	}()
	return &job
}

// Update changes the progress fields of a job.
func (m *JobManager) Update(id, stage string, done, total int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if st := m.jobs[id]; st != nil {
		st.job.Stage = stage
		st.job.Done = done
		st.job.Total = total
	}
}

// Report is a download.Reporter bound to a job.
func (m *JobManager) Report(id string) func(string, int64, int64) {
	return func(stage string, done, total int64) { m.Update(id, stage, done, total) }
}

// Cancel asks a job to stop.
func (m *JobManager) Cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.jobs[id]
	if st == nil {
		return false
	}
	st.cancel()
	return true
}

// Get returns a copy of one job.
func (m *JobManager) Get(id string) (Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st := m.jobs[id]
	if st == nil {
		return Job{}, false
	}
	return st.job, true
}

// List returns up to limit recent jobs, newest first.
func (m *JobManager) List(limit int) []Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.all) {
		limit = len(m.all)
	}
	out := make([]Job, 0, limit)
	for i := len(m.all) - 1; i >= 0 && len(out) < limit; i-- {
		if st := m.jobs[m.all[i]]; st != nil {
			out = append(out, st.job)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Started.After(out[j].Started) })
	return out
}
