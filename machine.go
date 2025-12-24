package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

type MachineState struct {
	OS         string
	Arch       string
	RootExists bool
	Runners    map[string]struct{}
}

func NewMachineState(rootDir string) (MachineState, error) {
	s := MachineState{
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		Runners: map[string]struct{}{},
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
		s.Runners[d.Name()] = struct{}{}
	}

	return s, nil
}

func supported(os, arch string) bool {
	switch strings.Join([]string{os, arch}, "/") {
	case "darwin/arm64":
		return true
	default:
		return false
	}
}
