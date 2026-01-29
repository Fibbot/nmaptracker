package testutil

import (
	"os"
	"testing"
)

func TestTempDir(t *testing.T) {
	dir := TempDir(t)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("temp dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("temp dir is not a directory: %s", dir)
	}
}
