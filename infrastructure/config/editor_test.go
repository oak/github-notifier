package config

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEditorCommand_ReturnsCorrectCommandForPlatform(t *testing.T) {
	path := "/home/user/.github-notifier.conf"
	cmd, args := editorCommand(path)

	switch runtime.GOOS {
	case "linux":
		assert.Equal(t, "xdg-open", cmd)
		assert.Equal(t, []string{path}, args)
	case "darwin":
		assert.Equal(t, "open", cmd)
		assert.Equal(t, []string{"-t", path}, args)
	case "windows":
		assert.Equal(t, "notepad", cmd)
		assert.Equal(t, []string{path}, args)
	default:
		assert.Empty(t, cmd)
		assert.Nil(t, args)
	}
}
