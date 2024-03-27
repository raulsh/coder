package pathman

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_splitOnPathToken(t *testing.T) {
	t.Parallel()
	t.Run("Empty", func(t *testing.T) {
		t.Parallel()
		before, section, after, err := splitOnPathToken([]byte(""))
		require.NoError(t, err)
		require.Empty(t, before)
		require.Empty(t, section)
		require.Empty(t, after)
	})
	t.Run("Missing", func(t *testing.T) {
		t.Parallel()
		before, section, after, err := splitOnPathToken([]byte(`# example
# content`))
		require.NoError(t, err)
		require.Equal(t, "# example\n# content", string(before))
		require.Empty(t, section)
		require.Empty(t, after)
	})
	t.Run("Exists", func(t *testing.T) {
		t.Parallel()
		before, section, after, err := splitOnPathToken([]byte(`# some comment
` + pathDelimiter + `
PATH="$HOME/wowza:$PATH"
# some other stuff
`))
		require.NoError(t, err)
		require.Equal(t, "# some comment\n", string(before))
		require.Equal(t, "# some other stuff\n", string(after))
		require.Equal(t, pathDelimiter+"\nPATH=\"$HOME/wowza:$PATH\"\n", string(section))
	})
	t.Run("NoEndNewline", func(t *testing.T) {
		t.Parallel()
		before, section, after, err := splitOnPathToken([]byte(`# some comment
` + pathDelimiter + `
wow`))
		require.NoError(t, err)
		require.Equal(t, "# some comment\n", string(before))
		require.Equal(t, pathDelimiter+"\nwow", string(section))
		require.Empty(t, string(after))
	})
	t.Run("NoSubsequentLine", func(t *testing.T) {
		t.Parallel()
		before, section, after, err := splitOnPathToken([]byte(`# some comment
` + pathDelimiter))
		require.NoError(t, err)
		require.Equal(t, "# some comment\n", string(before))
		require.Equal(t, pathDelimiter, string(section))
		// In this sample, there is no subsequent line... so the section is not found.
		require.Empty(t, string(after))
	})
}
