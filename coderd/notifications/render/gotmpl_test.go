package render

import (
	"testing"

	"github.com/coder/coder/v2/coderd/notifications/types"
	"github.com/stretchr/testify/require"
)

func TestGoTemplate(t *testing.T) {
	t.Parallel()

	const userEmail = "bob@xyz.com"

	tests := []struct {
		name           string
		in             string
		payload        types.MessagePayload
		expectedOutput string
		expectedErr    error
	}{
		{
			name:           "top-level variables are accessible and substituted",
			in:             "{{ .UserEmail }}",
			payload:        types.MessagePayload{UserEmail: userEmail},
			expectedOutput: userEmail,
			expectedErr:    nil,
		},
		{
			name: "input labels are accessible and substituted",
			in:   "{{ .Labels.user_email }}",
			payload: types.MessagePayload{Labels: map[string]string{
				"user_email": userEmail,
			}},
			expectedOutput: userEmail,
			expectedErr:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := GoTemplate(tc.in, tc.payload)
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.expectedErr)
			}

			require.Equal(t, tc.expectedOutput, out)
		})
	}
}
