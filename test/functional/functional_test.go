package functional

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	binaryName      = "validator"
	binaryPath      = "bin"
	functionalTests = "functional"
)

var (
	binaryFilepath            = filepath.Join(binaryPath, binaryName)
	projectRoot, _            = getProjectRoot()
	err                       error
	expectedSuccessExitCode   = 0
	expectedErrExitCode       = 1
	expectedValidationErrCode = 2
)

func TestMain(m *testing.M) {
	// Compile the CLI binary once
	cmd := exec.Command("go", "build", "-o", binaryFilepath, "github.com/Boeing/config-file-validator/cmd/validator")
	err := cmd.Run()
	if err != nil {
		// Fail the whole test suite if compilation fails
		panic(err)
	}

	// Add execute permissions to the binary
	err = os.Chmod(binaryFilepath, 0755)
	if err != nil {
		panic(err)
	}

	// Run the tests
	exitCode := m.Run()

	// Clean up the binary
	_, err = os.Stat(binaryFilepath)
	if err == nil {
		err = os.Remove(binaryFilepath)
		if err != nil {
			panic(err)
		}
	}
	os.Exit(exitCode)
}

func getProjectRoot() (string, error) {
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(b)
	projectRoot, err := filepath.Abs(filepath.Join(basepath, "..", ".."))

	return projectRoot, err
}

func TestHelp(t *testing.T) {
	cmd := exec.Command(binaryFilepath, "--help")
	output, err := cmd.CombinedOutput()

	assert.NoError(t, err, "The command should execute without errors")
	assert.Contains(t, string(output), "Usage:", "The output should contain the usage information")

	exitCode := cmd.ProcessState.ExitCode()
	assert.Equal(t, expectedSuccessExitCode, exitCode, "The command should exit with a success code")
}

func TestVersion(t *testing.T) {
	cmd := exec.Command(binaryFilepath, "--version")
	output, err := cmd.CombinedOutput()

	assert.NoError(t, err, "The command should execute without errors")
	assert.Contains(t, string(output), "validator version unknown", "The output should contain the version information")

	exitCode := cmd.ProcessState.ExitCode()
	assert.Equal(t, expectedSuccessExitCode, exitCode, "The command should exit with a success code")
}

func TestBadPath(t *testing.T) {
	cmd := exec.Command(binaryFilepath, "/badpath")
	output, err := cmd.CombinedOutput()

	assert.Error(t, err, "The command should execute with an error")
	// The error message has a timestamp, so we can't do an exact match
	assert.Contains(t, string(output), "An error occurred during CLI execution: unable to find files: stat /badpath: no such file or directory", "The output should contain the error message")

	exitCode := cmd.ProcessState.ExitCode()
	assert.Equal(t, expectedErrExitCode, exitCode, "The command should exit with an error code")
}

func TestInvalidFlag(t *testing.T) {
	cmd := exec.Command(binaryFilepath, "-v")
	output, err := cmd.CombinedOutput()

	assert.Error(t, err, "The command should execute with an error")
	assert.Contains(t, string(output), "flag provided but not defined: -v", "The output should contain the error message")

	exitCode := cmd.ProcessState.ExitCode()
	assert.Equal(t, expectedValidationErrCode, exitCode, "The command should exit with a validation error code")
}

func TestBasicValidation(t *testing.T) {
	fixturesPath := filepath.Join(projectRoot, "test", "fixtures")
	cmd := exec.Command(binaryFilepath, fixturesPath)
	output, err := cmd.CombinedOutput()

	assert.Error(t, err, "The command should execute with an error because there are failing files")
	assert.Contains(t, string(output), "bad.json", "The output should contain the name of a failing file")

	exitCode := cmd.ProcessState.ExitCode()
	assert.Equal(t, expectedErrExitCode, exitCode, "The command should exit with an error code")
}

func TestQuietFlag(t *testing.T) {
	fixturesPath := filepath.Join(projectRoot, "test", "fixtures")
	cmd := exec.Command(binaryFilepath, "--quiet", fixturesPath)
	output, err := cmd.CombinedOutput()

	assert.Error(t, err, "The command should execute with an error because there are failing files")
	assert.Empty(t, string(output), "The output should be empty when the --quiet flag is used")

	exitCode := cmd.ProcessState.ExitCode()
	assert.Equal(t, expectedErrExitCode, exitCode, "The command should exit with an error code")
}
