package testutil

import "testing"

// TempDir wraps t.TempDir for consistency and future shared setup.
func TempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}
