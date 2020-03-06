// Copyright 2020 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package root

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"gvisor.dev/gvisor/runsc/specutils"
)

// TestDoKill checks that when "runsc do..." is killed, all processes related
// to its execution are terminated. This ensures that parent death signal is
// propagate to the sandbox process correctly.
func TestDoKill(t *testing.T) {
	cmd := exec.Command(specutils.ExePath, "do", "sleep", "10000")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()

	desc, err := descedants(cmd.Process.Pid)
	if err != nil {
		t.Fatalf("error finding children: %v", err)
	}
	t.Logf("Found descedants: %v", desc)

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("failed to kill run process: %v", err)
	}
	t.Logf("Parent process killed (%d)", cmd.Process.Pid)

	// Check that all descendant processes are gone.
	for _, pid := range desc {
		_, err := syscall.Wait4(pid, nil, syscall.WNOHANG, nil)
		if err == nil {
			t.Errorf("child process %d didn't terminate", pid)
		}
		if !errors.Is(err, syscall.ECHILD) {
			t.Errorf("unexpected error waiting for %d: %v (%T)", pid, err, err)
		}
	}
}

func descedants(pid int) ([]int, error) {
	cmd := exec.Command("pgrep", "-P", strconv.Itoa(pid))
	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	ps, err := cmd.Process.Wait()
	if err != nil {
		return nil, err
	}
	if ps.ExitCode() == 1 {
		// No children found.
		return nil, nil
	}

	var children []int
	for _, line := range strings.Split(buf.String(), "\n") {
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		child, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		children = append(children, child)

		desc, err := descedants(child)
		if err != nil {
			return nil, err
		}
		children = append(children, desc...)
	}
	return children, nil
}
