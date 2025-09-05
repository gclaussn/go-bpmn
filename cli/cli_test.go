package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelp(t *testing.T) {
	assert := assert.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	rootCmd := newRootCmd(&Cli{e: e})

	rootCmd.SetArgs([]string{})
	assert.NoError(rootCmd.Execute())

	rootCmd.SetArgs([]string{"element"})
	assert.NoError(rootCmd.Execute())
	rootCmd.SetArgs([]string{"element-instance"})
	assert.NoError(rootCmd.Execute())
	rootCmd.SetArgs([]string{"incident"})
	assert.NoError(rootCmd.Execute())
	rootCmd.SetArgs([]string{"job"})
	assert.NoError(rootCmd.Execute())
	rootCmd.SetArgs([]string{"process"})
	assert.NoError(rootCmd.Execute())
	rootCmd.SetArgs([]string{"process-instance"})
	assert.NoError(rootCmd.Execute())
	rootCmd.SetArgs([]string{"task"})
	assert.NoError(rootCmd.Execute())
	rootCmd.SetArgs([]string{"variable"})
	assert.NoError(rootCmd.Execute())
	rootCmd.SetArgs([]string{"version"})
	assert.NoError(rootCmd.Execute())

	rootCmd.SetArgs([]string{"process", "create", "--help"})
	assert.NoError(rootCmd.Execute())
	rootCmd.SetArgs([]string{"set-time", "--help"})
	assert.NoError(rootCmd.Execute())
}
