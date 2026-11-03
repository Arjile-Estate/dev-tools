package devresult

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var safeJobPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)

func WriteArtifacts(root string, result Result, retainedLog string) (Result, error) {
	if !safeJobPattern.MatchString(result.Job) {
		return Result{}, fmt.Errorf("invalid DEV-RESULT job identifier %q", result.Job)
	}
	result.Job = uniqueJob(root, result.Job)
	result.Artifacts = artifactPaths(result.Job)
	result.DrillDown = drillDown(result)

	logPath := filepath.Join(root, filepath.FromSlash(result.Artifacts.Log))
	resultPath := filepath.Join(root, filepath.FromSlash(result.Artifacts.Result))
	if err := atomicWrite(logPath, []byte(retainedLog), 0o600); err != nil {
		return Result{}, fmt.Errorf("write DEV-RESULT log: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("marshal DEV-RESULT sidecar: %w", err)
	}
	data = append(data, '\n')
	if err := atomicWrite(resultPath, data, 0o600); err != nil {
		return Result{}, fmt.Errorf("write DEV-RESULT sidecar: %w", err)
	}
	return result, nil
}

func LoadArtifacts(root, job string) (Result, string, error) {
	if !safeJobPattern.MatchString(job) {
		return Result{}, "", fmt.Errorf("invalid DEV-RESULT job identifier %q", job)
	}
	resultPath := filepath.Join(root, ".build", "agent", "results", job+".json")
	data, err := os.ReadFile(resultPath)
	if err != nil {
		return Result{}, "", fmt.Errorf("read DEV-RESULT sidecar: %w", err)
	}
	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		return Result{}, "", fmt.Errorf("decode DEV-RESULT sidecar: %w", err)
	}
	if result.Schema != SchemaV1 || result.Job != job {
		return Result{}, "", fmt.Errorf("DEV-RESULT sidecar identity mismatch for %q", job)
	}
	logPath := filepath.Join(root, ".build", "agent", "logs", job+".log")
	logData, err := os.ReadFile(logPath)
	if err != nil {
		return Result{}, "", fmt.Errorf("read DEV-RESULT log: %w", err)
	}
	return result, string(logData), nil
}

func uniqueJob(root, requested string) string {
	job := requested
	for suffix := 2; ; suffix++ {
		resultPath := filepath.Join(root, ".build", "agent", "results", job+".json")
		logPath := filepath.Join(root, ".build", "agent", "logs", job+".log")
		if _, resultErr := os.Stat(resultPath); os.IsNotExist(resultErr) {
			if _, logErr := os.Stat(logPath); os.IsNotExist(logErr) {
				return job
			}
		}
		job = fmt.Sprintf("%s-%d", requested, suffix)
	}
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".dev-result-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
