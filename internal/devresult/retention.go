package devresult

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const LeaseSchemaV1 = "dev-tools/retention-lease/v1"

const retentionRemnantGrace = 5 * time.Minute

type Lease struct {
	Schema    string    `json:"schema"`
	Owner     string    `json:"owner"`
	RunIDs    []string  `json:"run_ids"`
	ExpiresAt time.Time `json:"expires_at"`
}

type RetentionPolicy struct {
	MaxAge   time.Duration
	MaxBytes int64
}

type RetentionReport struct {
	Removed []string
	Skipped bool
	Warning string
}

func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{MaxAge: 7 * 24 * time.Hour, MaxBytes: 1 << 30}
}

func ApplyRetention(root, currentRunID string, now time.Time, policy RetentionPolicy) (RetentionReport, error) {
	policy = policy.withDefaults()
	if currentRunID != "" && !safeJobPattern.MatchString(currentRunID) {
		return RetentionReport{}, fmt.Errorf("invalid current run id %q", currentRunID)
	}
	protected, warning, err := loadLiveLeases(root, now)
	if err != nil {
		return RetentionReport{}, err
	}
	if warning != "" {
		return RetentionReport{Skipped: true, Warning: warning}, nil
	}
	active, err := loadActiveRuns(root)
	if err != nil {
		return RetentionReport{}, err
	}
	for runID := range active {
		protected[runID] = true
	}
	if currentRunID != "" {
		protected[currentRunID] = true
	}

	jobs, err := scanRetentionJobs(root)
	if err != nil {
		return RetentionReport{}, err
	}
	sort.Slice(jobs, func(left, right int) bool {
		if jobs[left].modified.Equal(jobs[right].modified) {
			return jobs[left].id < jobs[right].id
		}
		return jobs[left].modified.Before(jobs[right].modified)
	})

	var total int64
	for _, job := range jobs {
		total += job.size
	}
	removed := make(map[string]bool)
	report := RetentionReport{}
	remove := func(job retentionJob) error {
		if err := removeRetentionJob(root, job); err != nil {
			return err
		}
		removed[job.id] = true
		total -= job.size
		report.Removed = append(report.Removed, job.id)
		return nil
	}
	for _, job := range jobs {
		if protected[job.id] || recentRetentionRemnant(job, now) || now.Sub(job.modified) <= policy.MaxAge {
			continue
		}
		if err := remove(job); err != nil {
			return report, err
		}
	}
	for _, job := range jobs {
		if total <= policy.MaxBytes {
			break
		}
		if protected[job.id] || removed[job.id] || recentRetentionRemnant(job, now) {
			continue
		}
		if err := remove(job); err != nil {
			return report, err
		}
	}
	if total > policy.MaxBytes {
		report.Warning = fmt.Sprintf("retention size remains %d bytes because current or leased runs are protected", total)
	}
	return report, nil
}

func (policy RetentionPolicy) withDefaults() RetentionPolicy {
	defaults := DefaultRetentionPolicy()
	if policy.MaxAge <= 0 {
		policy.MaxAge = defaults.MaxAge
	}
	if policy.MaxBytes <= 0 {
		policy.MaxBytes = defaults.MaxBytes
	}
	return policy
}

func loadLiveLeases(root string, now time.Time) (map[string]bool, string, error) {
	protected := make(map[string]bool)
	directory := filepath.Join(root, ".build", "agent", "leases")
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return protected, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("read retention leases: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("read retention lease %q: %w", entry.Name(), err)
		}
		lease, err := decodeLease(data)
		if err != nil {
			return protected, fmt.Sprintf("malformed lease %q: %v; retention skipped", entry.Name(), err), nil
		}
		if !lease.ExpiresAt.After(now) {
			continue
		}
		for _, runID := range lease.RunIDs {
			protected[runID] = true
		}
	}
	return protected, "", nil
}

func decodeLease(data []byte) (Lease, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var lease Lease
	if err := decoder.Decode(&lease); err != nil {
		return Lease{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Lease{}, errors.New("more than one JSON value")
		}
		return Lease{}, err
	}
	if lease.Schema != LeaseSchemaV1 {
		return Lease{}, fmt.Errorf("unsupported schema %q", lease.Schema)
	}
	if strings.TrimSpace(lease.Owner) == "" {
		return Lease{}, errors.New("owner is required")
	}
	if lease.ExpiresAt.IsZero() {
		return Lease{}, errors.New("expires_at is required")
	}
	for _, runID := range lease.RunIDs {
		if !safeJobPattern.MatchString(runID) {
			return Lease{}, fmt.Errorf("invalid run id %q", runID)
		}
	}
	return lease, nil
}

type retentionJob struct {
	id           string
	size         int64
	modified     time.Time
	verified     bool
	remnantPaths []string
}

var generatedJobPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*-[0-9]{8}T[0-9]{6}\.[0-9]{9}Z(?:-[A-Za-z0-9]+)*$`)
var atomicTemporaryPattern = regexp.MustCompile(`^\.dev-result-[0-9]+$`)

func recentRetentionRemnant(job retentionJob, now time.Time) bool {
	return len(job.remnantPaths) > 0 && now.Sub(job.modified) < retentionRemnantGrace
}

func loadActiveRuns(root string) (map[string]bool, error) {
	active := make(map[string]bool)
	directory := filepath.Join(root, ".build", "agent", "runs")
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return active, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan active validation runs: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || !safeJobPattern.MatchString(entry.Name()) {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(directory, entry.Name(), activeRunMarker))
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return nil, fmt.Errorf("read active validation run %q: %w", entry.Name(), readErr)
		}
		var record activeRunRecord
		if err := json.Unmarshal(data, &record); err != nil || record.Schema != activeRunSchemaV1 || record.PID <= 0 || record.CreatedAt.IsZero() {
			return nil, fmt.Errorf("invalid active validation marker for %q", entry.Name())
		}
		if processAlive(record.PID) {
			active[entry.Name()] = true
		}
	}
	return active, nil
}

func scanRetentionJobs(root string) ([]retentionJob, error) {
	resultsDirectory := filepath.Join(root, ".build", "agent", "results")
	entries, err := os.ReadDir(resultsDirectory)
	if errors.Is(err, os.ErrNotExist) {
		entries = nil
		err = nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan DEV-RESULT sidecars: %w", err)
	}
	jobsByID := make(map[string]*retentionJob)
	jobFor := func(id string) *retentionJob {
		job := jobsByID[id]
		if job == nil {
			job = &retentionJob{id: id}
			jobsByID[id] = job
		}
		return job
	}
	addArtifact := func(job *retentionJob, path string) error {
		size, modified, statErr := artifactStats(path)
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		job.size += size
		if modified.After(job.modified) {
			job.modified = modified
		}
		return nil
	}
	verified := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !safeJobPattern.MatchString(id) {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(resultsDirectory, entry.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("read DEV-RESULT sidecar %q: %w", id, readErr)
		}
		var result Result
		if json.Unmarshal(data, &result) != nil || result.Schema != SchemaV1 || result.Job != id {
			continue
		}
		job := jobFor(id)
		job.verified = true
		verified[id] = true
		paths := []string{
			filepath.Join(resultsDirectory, entry.Name()),
			filepath.Join(root, ".build", "agent", "logs", id+".log"),
			filepath.Join(root, ".build", "agent", "runs", id),
			filepath.Join(root, ".build", "agent", "evidence", id),
		}
		for _, path := range paths {
			if statErr := addArtifact(job, path); statErr != nil {
				return nil, fmt.Errorf("inspect retention artifact for %q: %w", id, statErr)
			}
		}
	}
	for _, directory := range []string{filepath.Join(root, ".build", "agent", "runs"), filepath.Join(root, ".build", "agent", "evidence")} {
		entries, readErr := os.ReadDir(directory)
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return nil, fmt.Errorf("scan orphan DEV-RESULT artifacts: %w", readErr)
		}
		for _, entry := range entries {
			id := entry.Name()
			if !entry.IsDir() || verified[id] || !safeJobPattern.MatchString(id) {
				continue
			}
			job := jobFor(id)
			if statErr := addArtifact(job, filepath.Join(directory, id)); statErr != nil {
				return nil, fmt.Errorf("inspect orphan DEV-RESULT artifact %q: %w", id, statErr)
			}
		}
	}
	addRemnant := func(id, path string) error {
		job := jobFor(id)
		if err := addArtifact(job, path); err != nil {
			return err
		}
		job.remnantPaths = append(job.remnantPaths, path)
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(resultsDirectory, name)
		switch {
		case atomicTemporaryPattern.MatchString(name):
			if err := addRemnant("temp-results-"+strings.TrimPrefix(name, ".dev-result-"), path); err != nil {
				return nil, fmt.Errorf("inspect temporary DEV-RESULT artifact %q: %w", name, err)
			}
		case strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".lock"):
			id := strings.TrimSuffix(strings.TrimPrefix(name, "."), ".lock")
			if safeJobPattern.MatchString(id) {
				if err := addRemnant(id, path); err != nil {
					return nil, fmt.Errorf("inspect DEV-RESULT reservation %q: %w", id, err)
				}
			}
		}
	}
	logsDirectory := filepath.Join(root, ".build", "agent", "logs")
	logEntries, readErr := os.ReadDir(logsDirectory)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return nil, fmt.Errorf("scan DEV-RESULT logs: %w", readErr)
	}
	for _, entry := range logEntries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(logsDirectory, name)
		switch {
		case atomicTemporaryPattern.MatchString(name):
			if err := addRemnant("temp-logs-"+strings.TrimPrefix(name, ".dev-result-"), path); err != nil {
				return nil, fmt.Errorf("inspect temporary DEV-RESULT log %q: %w", name, err)
			}
		case filepath.Ext(name) == ".log":
			id := strings.TrimSuffix(name, ".log")
			if !verified[id] && generatedJobPattern.MatchString(id) {
				if err := addRemnant(id, path); err != nil {
					return nil, fmt.Errorf("inspect orphan DEV-RESULT log %q: %w", id, err)
				}
			}
		}
	}
	jobs := make([]retentionJob, 0, len(jobsByID))
	for _, job := range jobsByID {
		jobs = append(jobs, *job)
	}
	return jobs, nil
}

func artifactStats(path string) (int64, time.Time, error) {
	var size int64
	var modified time.Time
	err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			size += info.Size()
		}
		if info.ModTime().After(modified) {
			modified = info.ModTime()
		}
		return nil
	})
	return size, modified, err
}

func removeRetentionJob(root string, job retentionJob) error {
	if !safeJobPattern.MatchString(job.id) {
		return fmt.Errorf("refuse to remove unsafe run id %q", job.id)
	}
	type retentionTarget struct {
		path      string
		recursive bool
	}
	paths := []retentionTarget{
		{path: filepath.Join(root, ".build", "agent", "runs", job.id), recursive: true},
		{path: filepath.Join(root, ".build", "agent", "evidence", job.id), recursive: true},
	}
	if job.verified {
		paths = append(paths,
			retentionTarget{path: filepath.Join(root, ".build", "agent", "results", job.id+".json")},
			retentionTarget{path: filepath.Join(root, ".build", "agent", "logs", job.id+".log")},
		)
	}
	for _, path := range job.remnantPaths {
		if !validRetentionRemnantPath(root, path) {
			return fmt.Errorf("refuse to remove unsafe retention remnant %q", path)
		}
		paths = append(paths, retentionTarget{path: path})
	}
	for _, target := range paths {
		var err error
		if target.recursive {
			err = os.RemoveAll(target.path)
		} else {
			err = os.Remove(target.path)
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
		}
		if err != nil {
			return fmt.Errorf("remove retention artifact %q: %w", target.path, err)
		}
	}
	return nil
}

func validRetentionRemnantPath(root, path string) bool {
	clean := filepath.Clean(path)
	name := filepath.Base(clean)
	resultsDirectory := filepath.Join(root, ".build", "agent", "results")
	logsDirectory := filepath.Join(root, ".build", "agent", "logs")
	switch filepath.Dir(clean) {
	case resultsDirectory:
		if atomicTemporaryPattern.MatchString(name) {
			return true
		}
		if strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".lock") {
			id := strings.TrimSuffix(strings.TrimPrefix(name, "."), ".lock")
			return safeJobPattern.MatchString(id)
		}
	case logsDirectory:
		if atomicTemporaryPattern.MatchString(name) {
			return true
		}
		if filepath.Ext(name) == ".log" {
			return generatedJobPattern.MatchString(strings.TrimSuffix(name, ".log"))
		}
	}
	return false
}
