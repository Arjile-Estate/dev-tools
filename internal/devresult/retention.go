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
	"sort"
	"strings"
	"time"
)

const LeaseSchemaV1 = "dev-tools/retention-lease/v1"

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
		if err := removeRetentionJob(root, job.id); err != nil {
			return err
		}
		removed[job.id] = true
		total -= job.size
		report.Removed = append(report.Removed, job.id)
		return nil
	}
	for _, job := range jobs {
		if protected[job.id] || now.Sub(job.modified) <= policy.MaxAge {
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
		if protected[job.id] || removed[job.id] {
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
	id       string
	size     int64
	modified time.Time
}

func scanRetentionJobs(root string) ([]retentionJob, error) {
	jobs := make(map[string]*retentionJob)
	locations := []struct {
		directory string
		extension string
		dirs      bool
	}{
		{directory: filepath.Join(root, ".build", "agent", "results"), extension: ".json"},
		{directory: filepath.Join(root, ".build", "agent", "logs"), extension: ".log"},
		{directory: filepath.Join(root, ".build", "agent", "runs"), dirs: true},
		{directory: filepath.Join(root, ".build", "agent", "evidence"), dirs: true},
	}
	for _, location := range locations {
		entries, err := os.ReadDir(location.directory)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("scan retention artifacts in %q: %w", location.directory, err)
		}
		for _, entry := range entries {
			if location.dirs != entry.IsDir() {
				continue
			}
			id := entry.Name()
			if !location.dirs {
				if filepath.Ext(id) != location.extension {
					continue
				}
				id = strings.TrimSuffix(id, location.extension)
			}
			if !safeJobPattern.MatchString(id) {
				continue
			}
			size, modified, err := artifactStats(filepath.Join(location.directory, entry.Name()))
			if err != nil {
				return nil, fmt.Errorf("inspect retention artifact for %q: %w", id, err)
			}
			job := jobs[id]
			if job == nil {
				job = &retentionJob{id: id}
				jobs[id] = job
			}
			job.size += size
			if modified.After(job.modified) {
				job.modified = modified
			}
		}
	}
	result := make([]retentionJob, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, *job)
	}
	return result, nil
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

func removeRetentionJob(root, runID string) error {
	if !safeJobPattern.MatchString(runID) {
		return fmt.Errorf("refuse to remove unsafe run id %q", runID)
	}
	paths := []struct {
		path      string
		recursive bool
	}{
		{path: filepath.Join(root, ".build", "agent", "results", runID+".json")},
		{path: filepath.Join(root, ".build", "agent", "logs", runID+".log")},
		{path: filepath.Join(root, ".build", "agent", "runs", runID), recursive: true},
		{path: filepath.Join(root, ".build", "agent", "evidence", runID), recursive: true},
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
