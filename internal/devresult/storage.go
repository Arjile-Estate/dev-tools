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

type RetainedSource struct {
	Kind    string
	Path    string
	Content string
}

type EvidenceBundle struct {
	AdapterResponse string
	AdapterStderr   string
	Sources         []RetainedSource
}

func WriteArtifacts(root string, result Result, retainedLog string) (Result, error) {
	budgets := budgetsFromResult(result)
	return writeArtifacts(root, result, retainedLog, budgets, false, EvidenceBundle{})
}

func WriteArtifactsForFormat(root string, result Result, retainedLog string, budgets Budgets, jsonFormat bool) (Result, error) {
	return writeArtifacts(root, result, retainedLog, budgets, jsonFormat, EvidenceBundle{})
}

func WriteArtifactsWithEvidenceForFormat(root string, result Result, retainedLog string, evidence EvidenceBundle, budgets Budgets, jsonFormat bool) (Result, error) {
	return writeArtifacts(root, result, retainedLog, budgets, jsonFormat, evidence)
}

func writeArtifacts(root string, result Result, retainedLog string, budgets Budgets, jsonFormat bool, evidence EvidenceBundle) (Result, error) {
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
	if evidence.AdapterResponse != "" {
		result.Artifacts.AdapterResponse = evidenceArtifactPath(result.Job, "adapter-response.json")
	}
	if evidence.AdapterStderr != "" {
		result.Artifacts.AdapterStderr = evidenceArtifactPath(result.Job, "adapter-stderr.log")
	}
	for _, source := range evidence.Sources {
		relative, cleanErr := cleanRelativeSourcePath(source.Path)
		if cleanErr != nil {
			return Result{}, cleanErr
		}
		result.Artifacts.Sources = append(result.Artifacts.Sources, evidenceArtifactPath(result.Job, filepath.ToSlash(filepath.Join("sources", relative))))
	}
	result.DrillDown = drillDown(result)
	if jsonFormat {
		compactJSON, compactErr := MarshalCompactJSON(result, budgets)
		if compactErr != nil {
			return Result{}, fmt.Errorf("compact DEV-RESULT JSON: %w", compactErr)
		}
		var compact Result
		if unmarshalErr := json.Unmarshal(compactJSON, &compact); unmarshalErr != nil {
			return Result{}, fmt.Errorf("decode compact DEV-RESULT JSON: %w", unmarshalErr)
		}
		result.Truncation = compact.Truncation
	} else {
		result.Truncation = Compact(result, budgets).Truncation
	}
	if originalJob != result.Job {
		retainedLog = strings.Replace(retainedLog, "job: "+originalJob+"\n", "job: "+result.Job+"\n", 1)
	}

	logPath := filepath.Join(root, filepath.FromSlash(result.Artifacts.Log))
	resultPath := filepath.Join(root, filepath.FromSlash(result.Artifacts.Result))
	evidenceDir := filepath.Join(root, ".build", "agent", "evidence", result.Job)
	cleanupEvidence := false
	defer func() {
		if cleanupEvidence {
			_ = os.RemoveAll(evidenceDir)
		}
	}()
	if evidence.AdapterResponse != "" {
		if err := atomicWrite(filepath.Join(root, filepath.FromSlash(result.Artifacts.AdapterResponse)), []byte(evidence.AdapterResponse), 0o600); err != nil {
			return Result{}, fmt.Errorf("write diagnostics adapter response: %w", err)
		}
		cleanupEvidence = true
	}
	if evidence.AdapterStderr != "" {
		if err := atomicWrite(filepath.Join(root, filepath.FromSlash(result.Artifacts.AdapterStderr)), []byte(evidence.AdapterStderr), 0o600); err != nil {
			return Result{}, fmt.Errorf("write diagnostics adapter stderr: %w", err)
		}
		cleanupEvidence = true
	}
	for index, source := range evidence.Sources {
		if err := atomicWrite(filepath.Join(root, filepath.FromSlash(result.Artifacts.Sources[index])), []byte(source.Content), 0o600); err != nil {
			return Result{}, fmt.Errorf("write diagnostic source %q: %w", source.Path, err)
		}
		cleanupEvidence = true
	}
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
	cleanupEvidence = false
	return result, nil
}

func evidenceArtifactPath(job, relative string) string {
	return filepath.ToSlash(filepath.Join(".build", "agent", "evidence", job, filepath.FromSlash(relative)))
}

func budgetsFromResult(result Result) Budgets {
	budgets := DefaultBudgets()
	if result.Truncation.Budget.Lines < 1 || result.Truncation.Budget.Bytes < 1 {
		return budgets
	}
	switch result.Outcome {
	case OutcomePass:
		budgets.Success = result.Truncation.Budget
	case OutcomeTimeout, OutcomeCancelled:
		budgets.Timeout = result.Truncation.Budget
	default:
		budgets.Failure = result.Truncation.Budget
	}
	return budgets
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
	retained := string(logData)
	appendArtifact := func(title, relative string) error {
		if relative == "" {
			return nil
		}
		data, readErr := readEvidenceArtifact(root, job, relative)
		if readErr != nil {
			return readErr
		}
		retained += fmt.Sprintf("\n\n--- %s ---\n%s", title, string(data))
		return nil
	}
	if err := appendArtifact("diagnostics adapter response", result.Artifacts.AdapterResponse); err != nil {
		return Result{}, "", err
	}
	if err := appendArtifact("diagnostics adapter stderr", result.Artifacts.AdapterStderr); err != nil {
		return Result{}, "", err
	}
	for _, source := range result.Artifacts.Sources {
		if err := appendArtifact("diagnostic source "+filepath.Base(source), source); err != nil {
			return Result{}, "", err
		}
	}
	return result, retained, nil
}

func readEvidenceArtifact(root, job, relative string) ([]byte, error) {
	clean := filepath.Clean(filepath.FromSlash(relative))
	expected := filepath.Join(".build", "agent", "evidence", job)
	contained, err := pathIsWithin(expected, clean)
	if err != nil || !contained {
		return nil, fmt.Errorf("invalid retained evidence path %q", relative)
	}
	data, err := os.ReadFile(filepath.Join(root, clean))
	if err != nil {
		return nil, fmt.Errorf("read retained evidence %q: %w", relative, err)
	}
	return data, nil
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
