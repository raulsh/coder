package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanPath(t *testing.T) {
	t.Parallel()
	parts := cleanPathParts("/home/test", []string{
		"/home/test",
		"/usr/local/bin",
		"/home/test",
		"/usr/local/test",
		"/home/test",
	})
	require.Len(t, parts, 2)
	require.Equal(t, []string{"/usr/local/bin", "/usr/local/test"}, parts)
}
