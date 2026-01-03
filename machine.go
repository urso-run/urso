package urso

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// MachineInspector defines the interface for inspecting the state of the local machine.
type MachineInspector interface {
	GetCurrentState(rootDir string) (MachineState, error)
	EnsureRootDirExists(rootDir string) error
	CreateTempDir(dir, pattern string) (string, error)
	RemoveAll(path string) error
	MkdirAll(path string) error
	LookPath(executable string) (string, error)
	Hostname() (string, error)
}

// FileSystemMachine is the production implementation of MachineInspector
// that inspects the local filesystem.
type FileSystemMachine struct{}

func (f *FileSystemMachine) GetCurrentState(rootDir string) (MachineState, error) {
	s := MachineState{
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		Runners: make(map[string]struct{}),
	}
	if !supported(s.OS, s.Arch) {
		return s, fmt.Errorf("unsupported os/arch: %s/%s", s.OS, s.Arch)
	}
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		s.RootExists = false
		return s, nil
	}
	s.RootExists = true

	dirs, err := os.ReadDir(rootDir)
	if err != nil {
		return s, fmt.Errorf("error reading root dir: %v", rootDir)
	}
	for _, d := range dirs {
		if d.IsDir() {
			s.Runners[d.Name()] = struct{}{}
		}
	}
	return s, nil
}

// EnsureRootDirExists creates the root directory if it doesn't exist.
func (f *FileSystemMachine) EnsureRootDirExists(rootDir string) error {
	return os.MkdirAll(rootDir, 0700)
}

// CreateTempDir creates a new temporary directory in the specified directory.
func (f *FileSystemMachine) CreateTempDir(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}

// RemoveAll removes a path and any children it contains.
func (f *FileSystemMachine) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// MkdirAll creates a directory path, along with any necessary parents.
func (f *FileSystemMachine) MkdirAll(path string) error {
	return os.MkdirAll(path, 0700)
}

// LookPath searches for an executable named file in the directories named by the PATH environment variable.
func (f *FileSystemMachine) LookPath(executable string) (string, error) {
	return exec.LookPath(executable)
}

// Hostname returns the host name reported by the kernel.
func (f *FileSystemMachine) Hostname() (string, error) {
	return os.Hostname()
}

func supported(os, arch string) bool {
	switch strings.Join([]string{os, arch}, "/") {
	case "darwin/arm64", "darwin/amd64", "linux/amd64", "linux/arm64":
		return true
	default:
		return false
	}
}
