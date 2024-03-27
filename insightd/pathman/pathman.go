package pathman

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/xerrors"

	"github.com/coder/coder/v2/agent/usershell"
)

const (
	pathDelimiter = "# Command Insights Binary Path"

	// Encompasses bash and sh
	profileConfig = ".profile"
	zshrcConfig   = ".zshrc"
	fishConfig    = ".config/fish/config.fish"
)

// PathsFromShell returns the current PATH from executing a shell.
//
// It's intentional we don't use Go environment variables here.
// We want system shells to have the `$PATH` from their configs,
// not from inheriting the Go environment.
func PathsFromShell(ctx context.Context) ([]string, error) {
	if runtime.GOOS == "windows" {
		cmd := exec.CommandContext(ctx, "cmd", "/C", "echo", "%PATH%")
		rawPath, err := cmd.CombinedOutput()
		if err != nil {
			return nil, err
		}
		pathParts := strings.Split(strings.TrimSpace(string(rawPath)), ";")
		return pathParts, nil
	}

	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	shell, err := usershell.Get(currentUser.Username)
	if err != nil {
		return nil, err
	}
	isFish := filepath.Base(shell) == "fish"
	cmd := exec.CommandContext(ctx, shell, "--login", "-c", "echo $PATH")
	rawPath, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	delimiter := ":"
	if isFish {
		delimiter = " "
	}
	pathParts := strings.Split(strings.TrimSpace(string(rawPath)), delimiter)
	return pathParts, nil
}

// Prepend mutates the commonly used script files to
// prepend a `$PATH` provided. It then creates a shell
// to ensure the new `$PATH` is used.
//
// The provided path should be the path to the directory
// that contains the binary that should be aliased.
func Prepend(ctx context.Context, fs afero.Fs, path string) error {
	if runtime.GOOS == "windows" {
		return prependWindows(ctx, path)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	// We want to create `~/.profile` if it doesn't exist.
	// This is the most common shell config profile.
	profile, err := fs.OpenFile(filepath.Join(homeDir, profileConfig), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer profile.Close()
	err = writePathTokenToFile(profile, fmt.Sprintf(`PATH="%s:$PATH"`, path))
	if err != nil {
		return err
	}

	// We don't want to create `~/.zshrc` if it doesn't exist.
	// This could pollute the users home directory.
	zsh, err := fs.OpenFile(filepath.Join(homeDir, zshrcConfig), os.O_RDWR, 0644)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		defer zsh.Close()
		err = writePathTokenToFile(zsh, fmt.Sprintf(`export PATH="%s:$PATH"`, path))
		if err != nil {
			return err
		}
	}

	// We don't want to create a fish config if it doesn't exist.
	// This could pollute the users home directory.
	fish, err := fs.OpenFile(filepath.Join(homeDir, fishConfig), os.O_RDWR, 0644)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		defer fish.Close()
		// This is special fish syntax!
		err = writePathTokenToFile(fish, fmt.Sprintf(`set -px --path PATH %q`, path))
		if err != nil {
			return err
		}
	}

	return nil
}

// Reset resets the commonly used script files to
// their original state.
func Reset(ctx context.Context, fs afero.Fs, path string) error {
	if runtime.GOOS == "windows" {
		return removePathOnWindows(ctx, path)
	}
	currentUser, err := user.Current()
	if err != nil {
		return err
	}
	for _, config := range []string{profileConfig, zshrcConfig, fishConfig} {
		err = removePathTokenFromFile(fs, filepath.Join(currentUser.HomeDir, config))
		if err != nil {
			return err
		}
	}
	return nil
}

// writePathTokenToFile writes the provided line to the provided file.
func writePathTokenToFile(file afero.File, line string) error {
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	before, found, after, err := splitOnPathToken(content)
	if err != nil {
		return err
	}
	section := []byte(pathDelimiter + "\n" + line + "\n")
	if bytes.Equal(found, section) {
		return nil
	}
	err = file.Truncate(int64(len(before)))
	if err != nil {
		return err
	}
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	_, err = file.Write(append(section, after...))
	return err
}

// removePathTokenFromFile removes the path token from the provided file.
func removePathTokenFromFile(fs afero.Fs, path string) error {
	file, err := fs.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	before, found, after, err := splitOnPathToken(content)
	if err != nil {
		return err
	}
	if len(found) == 0 {
		return nil
	}
	err = file.Truncate(int64(len(before)))
	if err != nil {
		return err
	}
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	_, err = file.Write(after)
	return err
}

// splitOnPathToken splits the provided data into the
// path token plus an additional line for actually setting
// the path.
//
// To restore a file, simply add the `before` and `after`.
func splitOnPathToken(data []byte) (before, section, after []byte, err error) {
	startCount := bytes.Count(data, []byte(pathDelimiter))
	if startCount > 1 {
		return nil, nil, nil, xerrors.New("found more than one start token")
	}
	start := bytes.Index(data, []byte(pathDelimiter))
	if start == -1 {
		// There is no section or after... it's all before!
		return data, nil, nil, nil
	}
	// Get until the end of the path token. Add 1 to include the newline.
	end := start + bytes.IndexByte(data[start:], '\n') + 1
	// Get until the end of the subsequent line. Add 1 to include the newline.
	lastLineIndex := bytes.IndexByte(data[end:], '\n')
	if lastLineIndex != -1 {
		end += lastLineIndex + 1
	} else {
		end = len(data)
	}
	return data[:start], data[start:end], data[end:], nil
}

// prependWindows prepends the provided path to the user's
// PATH environment variable. It does this by reading the
// current PATH, removing the provided path if it exists,
// and then prepending the provided path.
func prependWindows(ctx context.Context, path string) error {
	pathParts, err := PathsFromShell(ctx)
	if err != nil {
		return err
	}
	for index, part := range pathParts {
		if index == 0 && part == path {
			// The path is already in the front!
			return nil
		}
		if part == path {
			// Remove the path from the list
			pathParts = append(pathParts[:index], pathParts[index+1:]...)
			break
		}
	}
	pathParts = append([]string{path}, pathParts...)
	//nolint:gosec
	cmd := exec.CommandContext(ctx, "setx", "PATH", strings.Join(pathParts, ";"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return xerrors.Errorf("%w: %s", err, output)
	}
	return nil
}

// removePathOnWindows removes the provided path from the user's
// PATH environment variable. It does this by reading the current
// PATH, removing the provided path if it exists, and then setting
// the new PATH.
func removePathOnWindows(ctx context.Context, path string) error {
	pathParts, err := PathsFromShell(ctx)
	if err != nil {
		return err
	}
	for index, part := range pathParts {
		if part != path {
			continue
		}
		pathParts = append(pathParts[:index], pathParts[index+1:]...)
		break
	}
	//nolint:gosec
	cmd := exec.CommandContext(ctx, "setx", "PATH", strings.Join(pathParts, ";"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return xerrors.Errorf("%w: %s", err, output)
	}
	return nil
}
