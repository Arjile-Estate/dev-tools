package devresult

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type AdapterOptions struct {
	Argv       []string
	WorkingDir string
	Request    DiagnosticsRequest
	Limits     Limits
}

type AdapterResult struct {
	Response        *DiagnosticsResponse
	Stdout          []byte
	Stderr          []byte
	Err             error
	TimedOut        bool
	Cancelled       bool
	StdoutTruncated bool
	StderrTruncated bool
}

func ExecuteAdapter(ctx context.Context, options AdapterOptions) AdapterResult {
	limits := options.Limits.withDefaults()
	if err := validateDiagnosticsRequest(options.Request); err != nil {
		return AdapterResult{Err: err}
	}
	if len(options.Argv) == 0 || options.Argv[0] == "" {
		return AdapterResult{Err: errors.New("diagnostics adapter argv is empty")}
	}
	if err := ctx.Err(); err != nil {
		return AdapterResult{Err: fmt.Errorf("diagnostics adapter cancelled: %w", err), Cancelled: errors.Is(err, context.Canceled)}
	}

	request, err := json.Marshal(options.Request)
	if err != nil {
		return AdapterResult{Err: fmt.Errorf("encode diagnostics request: %w", err)}
	}
	request = append(request, '\n')

	runContext, cancel := context.WithTimeout(ctx, limits.Timeout)
	defer cancel()
	command := exec.CommandContext(runContext, options.Argv[0], options.Argv[1:]...)
	configureAdapterProcess(command)
	command.WaitDelay = 100 * time.Millisecond
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		return killAdapterProcessTree(command.Process)
	}
	command.Dir = options.WorkingDir
	command.Stdin = bytes.NewReader(request)
	command.Env = environmentWithOverrides(os.Environ(), map[string]string{
		RunIDEnvironment:  options.Request.RunID,
		RunDirEnvironment: options.Request.RunDir,
	})
	stdout := newBoundedBuffer(limits.StdoutBytes)
	stderr := newBoundedBuffer(limits.StderrBytes)
	command.Stdout = stdout
	command.Stderr = stderr

	runErr := command.Run()
	result := AdapterResult{
		Stdout:          stdout.Bytes(),
		Stderr:          stderr.Bytes(),
		StdoutTruncated: stdout.Truncated(),
		StderrTruncated: stderr.Truncated(),
	}
	if ctx.Err() != nil {
		result.Cancelled = errors.Is(ctx.Err(), context.Canceled)
		result.Err = fmt.Errorf("diagnostics adapter cancelled: %w", ctx.Err())
		return result
	}
	if errors.Is(runContext.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
		result.Err = fmt.Errorf("diagnostics adapter exceeded timeout %s: %w", limits.Timeout, context.DeadlineExceeded)
		return result
	}
	if result.StdoutTruncated {
		result.Err = fmt.Errorf("diagnostics adapter standard output limit of %d bytes exceeded", limits.StdoutBytes)
		return result
	}
	if runErr != nil {
		result.Err = fmt.Errorf("run diagnostics adapter %q: %w", options.Argv[0], runErr)
		return result
	}

	response, err := decodeDiagnosticsResponse(result.Stdout)
	if err != nil {
		result.Err = err
		return result
	}
	if err := ValidateDiagnosticsResponse(response, limits); err != nil {
		result.Err = fmt.Errorf("invalid diagnostics adapter response: %w", err)
		return result
	}
	result.Response = &response
	return result
}

func environmentWithOverrides(base []string, overrides map[string]string) []string {
	environment := make([]string, 0, len(base)+len(overrides))
	for _, item := range base {
		name, _, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		replaced := false
		for override := range overrides {
			if strings.EqualFold(name, override) {
				replaced = true
				break
			}
		}
		if !replaced {
			environment = append(environment, item)
		}
	}
	for name, value := range overrides {
		environment = append(environment, name+"="+value)
	}
	return environment
}

func validateDiagnosticsRequest(request DiagnosticsRequest) error {
	if request.Schema != DiagnosticsRequestSchemaV1 {
		return fmt.Errorf("unsupported diagnostics request schema %q", request.Schema)
	}
	if request.RunID == "" {
		return errors.New("diagnostics request run_id is required")
	}
	if request.RunDir == "" {
		return errors.New("diagnostics request run_dir is required")
	}
	if request.Command == "" {
		return errors.New("diagnostics request command is required")
	}
	if request.DurationMS < 0 {
		return errors.New("diagnostics request duration_ms cannot be negative")
	}
	return nil
}

func decodeDiagnosticsResponse(data []byte) (DiagnosticsResponse, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var response DiagnosticsResponse
	if err := decoder.Decode(&response); err != nil {
		return DiagnosticsResponse{}, fmt.Errorf("decode diagnostics adapter response: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return DiagnosticsResponse{}, errors.New("diagnostics adapter wrote more than one JSON response")
		}
		return DiagnosticsResponse{}, fmt.Errorf("decode trailing diagnostics adapter output: %w", err)
	}
	return response, nil
}

type boundedBuffer struct {
	buffer    bytes.Buffer
	limit     int64
	truncated bool
}

func newBoundedBuffer(limit int64) *boundedBuffer {
	return &boundedBuffer{limit: limit}
}

func (buffer *boundedBuffer) Write(data []byte) (int, error) {
	written := len(data)
	remaining := buffer.limit - int64(buffer.buffer.Len())
	if remaining > 0 {
		keep := int64(len(data))
		if keep > remaining {
			keep = remaining
		}
		_, _ = buffer.buffer.Write(data[:int(keep)])
	}
	if int64(len(data)) > remaining {
		buffer.truncated = true
	}
	return written, nil
}

func (buffer *boundedBuffer) Bytes() []byte {
	return append([]byte(nil), buffer.buffer.Bytes()...)
}

func (buffer *boundedBuffer) Truncated() bool {
	return buffer.truncated
}
