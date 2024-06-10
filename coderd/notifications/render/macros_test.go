package render

import (
	"net/url"
	"testing"

	"github.com/coder/coder/v2/codersdk"
	"github.com/coder/serpent"
	"github.com/stretchr/testify/require"
)

func TestMacros(t *testing.T) {
	t.Parallel()

	const accessURL = "https://xyz.com"
	url, err := url.Parse("https://xyz.com")
	require.NoError(t, err)

	tests := []struct {
		name           string
		in             string
		cfg            codersdk.DeploymentValues
		expectedOutput string
		expectedErr    error
	}{
		{
			name: "ACCESS_URL",
			in:   "[ACCESS_URL]/workspaces",
			cfg: codersdk.DeploymentValues{
				AccessURL: *serpent.URLOf(url),
			},
			expectedOutput: accessURL + "/workspaces",
			expectedErr:    nil,
		},
		{
			name: "ACCESS_URL multiple",
			in:   "[ACCESS_URL] is [ACCESS_URL]",
			cfg: codersdk.DeploymentValues{
				AccessURL: *serpent.URLOf(url),
			},
			expectedOutput: accessURL + " is " + accessURL,
			expectedErr:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := Macros(tc.cfg, tc.in)
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.expectedErr)
			}

			require.Equal(t, tc.expectedOutput, out)
		})
	}
}
