package claude_test

import (
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const claudeJSON = `{"numStartups":3,"mcpServers":{` +
	`"splunk":{"type":"stdio","command":"npx","args":["-y","splunk-mcp"],"env":{"TOKEN":"<scrubbed>"}},` +
	`"datadog":{"type":"http","url":"https://mcp.datadoghq.com/api","headers":{"DD-KEY":"<scrubbed>"}},` +
	`"local":{"command":"/usr/bin/local-mcp"}},` +
	`"projects":{` +
	`"/home/avesta/workspaces/mullet":{"allowedTools":[],"mcpServers":{` +
	`"atlassian":{"type":"http","url":"https://mcp.atlassian.com/v1/sse"},` +
	`"datadog":{"type":"sse","url":"https://mcp.datadoghq.com/sse"}}},` +
	`"/home/avesta/other":{"allowedTools":[]}}}`

func TestReadMCPConfig(t *testing.T) {
	t.Run("when the file holds global and per-project servers", func(t *testing.T) {
		config, err := claude.ReadMCPConfig(writeFile(t, t.TempDir(), ".claude.json", claudeJSON))
		require.NoError(t, err)

		t.Run("it should list the global servers sorted by name", func(t *testing.T) {
			assert.Equal(t, []string{"datadog", "local", "splunk"}, serverNames(config.Global))
		})

		t.Run("it should read the type of a typed server", func(t *testing.T) {
			assert.Equal(t, "http", config.Global[0].Type)
		})

		t.Run("it should infer stdio for a server with a command and no type", func(t *testing.T) {
			assert.Equal(t, "stdio", config.Global[1].Type)
		})

		t.Run("it should read the url of an http server", func(t *testing.T) {
			assert.Equal(t, "https://mcp.datadoghq.com/api", config.Global[0].URL)
		})

		t.Run("it should read the command and args of a stdio server", func(t *testing.T) {
			assert.Equal(t, []string{"npx", "-y", "splunk-mcp"}, append([]string{config.Global[2].Command}, config.Global[2].Args...))
		})

		t.Run("it should read a project's servers", func(t *testing.T) {
			assert.Equal(t, []string{"atlassian", "datadog"}, serverNames(config.Projects["/home/avesta/workspaces/mullet"]))
		})

		t.Run("it should not list a project without servers", func(t *testing.T) {
			assert.NotContains(t, config.Projects, "/home/avesta/other")
		})

		t.Run("and a project is asked for its servers", func(t *testing.T) {
			servers := config.ForProject("/home/avesta/workspaces/mullet")

			t.Run("it should merge global and project servers by name", func(t *testing.T) {
				assert.Equal(t, []string{"atlassian", "datadog", "local", "splunk"}, serverNames(servers))
			})

			t.Run("it should let the project definition win", func(t *testing.T) {
				assert.Equal(t, "sse", servers[1].Type)
			})
		})

		t.Run("and an unknown project is asked for its servers", func(t *testing.T) {
			t.Run("it should return the global servers", func(t *testing.T) {
				assert.Equal(t, serverNames(config.Global), serverNames(config.ForProject("/nowhere")))
			})
		})
	})

	t.Run("when the file does not exist", func(t *testing.T) {
		config, err := claude.ReadMCPConfig(filepath.Join(t.TempDir(), ".claude.json"))

		t.Run("it should not return an error", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should return an empty config", func(t *testing.T) {
			assert.Empty(t, config.Global)
		})
	})

	t.Run("when the file is not valid JSON", func(t *testing.T) {
		_, err := claude.ReadMCPConfig(writeFile(t, t.TempDir(), ".claude.json", `{"mcpServers":`))

		t.Run("it should return an error", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}

func serverNames(servers []claude.MCPServer) []string {
	names := make([]string, 0, len(servers))
	for _, s := range servers {
		names = append(names, s.Name)
	}
	return names
}
