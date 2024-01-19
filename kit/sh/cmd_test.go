package sh_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/quickfeed/quickfeed/kit/sh"
)

func TestRun(t *testing.T) {
	if err := sh.Run("cat doesnotexist.txt"); err == nil {
		t.Error("expected: exit status 1")
	}
}

func TestOutput(t *testing.T) {
	s, err := sh.Output("ls -la")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(s)
}

func TestLintAG(t *testing.T) {
	// check formatting using goimports
	s, err := sh.Output("golangci-lint run --tests=false --disable-all --enable goimports")
	if err != nil {
		t.Error(err)
	}
	if s != "" {
		t.Error("Formatting check failed: goimports")
	}

	// check for TODO/BUG/FIXME comments using godox
	s, err = sh.Output("golangci-lint run --tests=false --disable-all --enable godox")
	if err == nil {
		t.Error("Expected golangci-lint to return error with 'exit status 1'")
	}
	if s == "" {
		t.Error("Expected golangci-lint to return message 'TODO(meling) test golangci-lint godox check for TODO items'")
	}

	// check many things: probably too aggressive for DAT320
	s, err = sh.Output("golangci-lint run --tests=true --disable structcheck --disable unused --disable deadcode --disable varcheck")
	if err != nil {
		t.Error(err)
	}
	if s != "" {
		fmt.Println(s)
	}
}

func TestRunRaceTest(t *testing.T) {
	tests := []struct {
		testName         string
		expectedRace     bool
		expectedOutput   string
		unexpectedOutput string
	}{
		{
			testName:         "TestWithDataRace",
			expectedRace:     true,
			expectedOutput:   "WARNING: DATA RACE",
			unexpectedOutput: "PASS",
		},
		{
			testName:         "TestWithoutDataRace",
			expectedRace:     false,
			expectedOutput:   "PASS",
			unexpectedOutput: "WARNING: DATA RACE",
		},
		{
			testName:         "TestThatDoesNotExist",
			expectedRace:     false,
			expectedOutput:   "warning: no tests to run",
			unexpectedOutput: "WARNING: DATA RACE",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			output, race := sh.RunRaceTest(tt.testName)
			if (race && !tt.expectedRace) || (!race && tt.expectedRace) {
				prefix := "Expected"
				if race {
					prefix = "Unexpected"
				}
				t.Errorf("%s data race warning from %s", prefix, tt.testName)
			}
			expectedContains := strings.Contains(output, tt.expectedOutput)
			unexpectedContains := strings.Contains(output, tt.unexpectedOutput)
			if !expectedContains {
				t.Errorf("Expected output with '%s' from %s", tt.expectedOutput, tt.testName)
				t.Log(output)
			}
			if unexpectedContains {
				t.Errorf("Unexpected output with '%s' from %s", tt.unexpectedOutput, tt.testName)
				t.Log(output)
			}
		})
	}
}
