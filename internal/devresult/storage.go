package devresult

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var safeJobPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*$`)

func WriteArtifacts(root string, result Result, retainedLog string) (Result, error) {
	if !safeJobPattern.MatchString(result.Job) {
		return Result{}, fmt.Errorf("invalid DEV-RESULT job identifier %q", result.Job)
	}
	originalJob := result.Job
	reservedJob, release, err := reserveJob(root, originalJob)
	if err != nil {
		return Result{}, fmt.Errorf("reserve DEV-RESULT job: %w", err)
	}
	defer release()
	result.Job = reservedJob
	result.Artifacts = artifactPaths(result.Job)
	result.DrillDown = drillDown(result)
	budgets := DefaultBudgets()
	switch result.Outcome {
	case OutcomePass:
		budgets.Success = result.Truncation.Budget
	case OutcomeTimeout, OutcomeCancelled:
		budgets.Timeout = result.Truncation.Budget
	default:
		budgets.Failure = result.Truncation.Budget
	}
	result.Truncation = Compact(result, budgets).Truncation
	if originalJob != result.Job {
		retainedLog = strings.Replace(retainedLog, "job: "+originalJob+"\n", "job: "+result.Job+"\n", 1)
	}

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
		_ = os.Remove(logPath)
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

func reserveJob(root, requested string) (string, func(), error) {
	resultDir := filepath.Join(root, ".build", "agent", "results")
	logDir := filepath.Join(root, ".build", "agent", "logs")
	if err := os.MkdirAll(resultDir, 0o700); err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return "", nil, err
	}
	for suffix := 2; ; suffix++ {
		job := requested
		if suffix > 2 {
			job = fmt.Sprintf("%s-%d", requested, suffix-1)
		}
		lockPath := filepath.Join(resultDir, "."+job+".lock")
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		if closeErr := lock.Close(); closeErr != nil {
			_ = os.Remove(lockPath)
			return "", nil, closeErr
		}
		resultPath := filepath.Join(root, ".build", "agent", "results", job+".json")
		logPath := filepath.Join(root, ".build", "agent", "logs", job+".log")
		_, resultErr := os.Stat(resultPath)
		_, logErr := os.Stat(logPath)
		if os.IsNotExist(resultErr) && os.IsNotExist(logErr) {
			return job, func() { _ = os.Remove(lockPath) }, nil
		}
		_ = os.Remove(lockPath)
		if resultErr != nil && !os.IsNotExist(resultErr) {
			return "", nil, resultErr
		}
		if logErr != nil && !os.IsNotExist(logErr) {
			return "", nil, logErr
		}
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
