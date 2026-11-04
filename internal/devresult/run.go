package devresult

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	RunIDEnvironment  = "DEV_TOOLS_RUN_ID"
	RunDirEnvironment = "DEV_TOOLS_RUN_DIR"
)

type ValidationRun struct {
	ID  string
	Dir string
}

type ValidatedSource struct {
	Kind         string
	Path         string
	AbsolutePath string
	Size         int64
}

func CreateValidationRun(root, command string, now time.Time) (ValidationRun, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return ValidationRun{}, fmt.Errorf("resolve project root: %w", err)
	}
	runsDir := filepath.Join(root, ".build", "agent", "runs")
	if err := os.MkdirAll(runsDir, 0o700); err != nil {
		return ValidationRun{}, fmt.Errorf("create validation runs directory: %w", err)
	}
	prefix := now.UTC().Format("20060102T150405.000000000Z") + "-" + safeRunLabel(command) + "-"
	dir, err := os.MkdirTemp(runsDir, prefix)
	if err != nil {
		return ValidationRun{}, fmt.Errorf("allocate validation run: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return ValidationRun{}, fmt.Errorf("secure validation run: %w", err)
	}
	return ValidationRun{ID: filepath.Base(dir), Dir: dir}, nil
}

func safeRunLabel(command string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(command) {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			builder.WriteRune(character)
		case builder.Len() > 0 && !strings.HasSuffix(builder.String(), "-"):
			builder.WriteByte('-')
		}
		if builder.Len() >= 32 {
			break
		}
	}
	label := strings.Trim(builder.String(), "-")
	if label == "" {
		return "run"
	}
	return label
}

func (run ValidationRun) Environment() map[string]string {
	return map[string]string{RunIDEnvironment: run.ID, RunDirEnvironment: run.Dir}
}

func (run ValidationRun) HasFreshEvidence(patterns []string) (bool, error) {
	matchers := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		matcher, err := compileEvidencePattern(pattern)
		if err != nil {
			return false, err
		}
		matchers = append(matchers, matcher)
	}
	found := false
	err := filepath.WalkDir(run.Dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if found || path == run.Dir || entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(run.Dir, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		for _, matcher := range matchers {
			if matcher.MatchString(relative) {
				found = true
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("probe fresh evidence: %w", err)
	}
	return found, nil
}

func compileEvidencePattern(pattern string) (*regexp.Regexp, error) {
	if pattern == "" || strings.ContainsRune(pattern, '\x00') || strings.Contains(pattern, "\\") || strings.HasPrefix(pattern, "/") {
		return nil, fmt.Errorf("unsafe evidence pattern %q", pattern)
	}
	clean := filepath.Clean(filepath.FromSlash(pattern))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("unsafe evidence pattern %q", pattern)
	}
	var expression strings.Builder
	expression.WriteByte('^')
	for index := 0; index < len(pattern); {
		switch pattern[index] {
		case '*':
			if index+1 < len(pattern) && pattern[index+1] == '*' {
				index += 2
				if index < len(pattern) && pattern[index] == '/' {
					index++
					expression.WriteString("(?:.*/)?")
				} else {
					expression.WriteString(".*")
				}
			} else {
				index++
				expression.WriteString("[^/]*")
			}
		case '?':
			index++
			expression.WriteString("[^/]")
		case '[':
			return nil, fmt.Errorf("unsafe evidence pattern %q: character classes are not supported", pattern)
		default:
			expression.WriteString(regexp.QuoteMeta(string(pattern[index])))
			index++
		}
	}
	expression.WriteByte('$')
	matcher, err := regexp.Compile(expression.String())
	if err != nil {
		return nil, fmt.Errorf("invalid evidence pattern %q: %w", pattern, err)
	}
	return matcher, nil
}

func (run ValidationRun) ValidateSources(sources []DiagnosticSource, limits Limits) ([]ValidatedSource, error) {
	limits = limits.withDefaults()
	runDir, err := filepath.Abs(run.Dir)
	if err != nil {
		return nil, fmt.Errorf("resolve run directory: %w", err)
	}
	resolvedRunDir, err := filepath.EvalSymlinks(runDir)
	if err != nil {
		return nil, fmt.Errorf("resolve run directory symlinks: %w", err)
	}
	validated := make([]ValidatedSource, 0, len(sources))
	var total int64
	for _, source := range sources {
		relative, err := cleanRelativeSourcePath(source.Path)
		if err != nil {
			return nil, err
		}
		absolute := filepath.Join(runDir, relative)
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return nil, fmt.Errorf("resolve diagnostic source %q: %w", source.Path, err)
		}
		contained, err := pathIsWithin(resolvedRunDir, resolved)
		if err != nil {
			return nil, fmt.Errorf("check diagnostic source %q: %w", source.Path, err)
		}
		if !contained {
			return nil, fmt.Errorf("diagnostic source %q symlink escapes run directory", source.Path)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, fmt.Errorf("inspect diagnostic source %q: %w", source.Path, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("diagnostic source %q is not a regular file", source.Path)
		}
		if info.Size() > limits.PerSourceBytes {
			return nil, fmt.Errorf("diagnostic source %q exceeds source size limit of %d bytes", source.Path, limits.PerSourceBytes)
		}
		if info.Size() > limits.TotalSourceBytes-total {
			return nil, fmt.Errorf("diagnostic sources exceed total source size limit of %d bytes", limits.TotalSourceBytes)
		}
		total += info.Size()
		validated = append(validated, ValidatedSource{Kind: source.Kind, Path: filepath.ToSlash(relative), AbsolutePath: resolved, Size: info.Size()})
	}
	return validated, nil
}

func cleanRelativeSourcePath(path string) (string, error) {
	if path == "" || strings.ContainsRune(path, '\x00') || strings.Contains(path, "\\") {
		return "", fmt.Errorf("diagnostic source %q escapes run directory", path)
	}
	relative := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(relative) || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("diagnostic source %q escapes run directory", path)
	}
	return relative, nil
}

func pathIsWithin(root, candidate string) (bool, error) {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false, err
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative), nil
}
