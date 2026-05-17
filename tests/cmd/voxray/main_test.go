package main_test

import (
	"os/exec"
	"testing"
)

func TestBuildVoxray(t *testing.T) {
	cmd := exec.Command("go", "build", "./cmd/voxray")
	cmd.Dir = "../../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/voxray: %v\n%s", err, out)
	}
}
