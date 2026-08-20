package integration

import "testing"

func skipIfNotIntegration(t testing.TB) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}
