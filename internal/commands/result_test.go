package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"dev-tools/internal/devresult"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeResultFixture(t *testing.T, root string) devresult.Result {
	t.Helper()
	result, retained := devresult.Build(devresult.Input{
		Job:               "build-fixed-001",
		CommandName:       "build",
		NormalizedCommand: "dev-tools build",
		ExitCode:          42,
		Stdout:            "generated four files\n",
		Stderr:            "error: compilation failed\nwarning: cache unavailable\n",
	}, devresult.DefaultBudgets(), nil)
	written, err := devresult.WriteArtifacts(root, result, retained)
	require.NoError(t, err)
	return written
}

func TestHandleResultCommandReadsArtifactsWithoutExecutingWork(t *testing.T) {
	root := t.TempDir()
	fixture := writeResultFixture(t, root)

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "compact", args: []string{fixture.Job}, want: "DEV-RESULT v1 FAIL"},
		{name: "full", args: []string{fixture.Job, "--full"}, want: "generated four files"},
		{name: "failures", args: []string{fixture.Job, "--failures"}, want: "error: compilation failed"},
		{name: "warnings", args: []string{fixture.Job, "--warnings"}, want: "warning: cache unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			var output bytes.Buffer
			cmd.SetOut(&output)
			require.NoError(t, HandleResultCommand(cmd, tc.args, root))
			assert.Contains(t, output.String(), tc.want)
		})
	}

	assert.FileExists(t, filepath.Join(root, filepath.FromSlash(fixture.Artifacts.Result)))
}

func TestHandleResultCommandRejectsPipeOrRunnerSyntax(t *testing.T) {
	cmd := &cobra.Command{}
	for _, args := range [][]string{{}, {"--", "dev-tools", "test"}, {"job", "--run"}} {
		err := HandleResultCommand(cmd, args, t.TempDir())
		require.Error(t, err)
	}
}

func TestResultBuiltInIsRegistered(t *testing.T) {
	assert.True(t, IsBuiltInCommand("result"))
	assert.Contains(t, GetBuiltInCommandNames(), "result")
	_ = os.ErrNotExist
}
