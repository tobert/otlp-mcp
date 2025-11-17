package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockFsUtils struct {
	executable    string
	executableErr error
	statMap       map[string]os.FileInfo
	statErr       error
	readFileMap   map[string][]byte
	readFileErr   error
	homeDir       string
	homeDirErr    error
	cwd           string
	cwdErr        error
	lookPathMap   map[string]string
	lookPathErr   error
}

func (m *mockFsUtils) Executable() (string, error) { return m.executable, m.executableErr }
func (m *mockFsUtils) Stat(name string) (os.FileInfo, error) {
	if info, ok := m.statMap[name]; ok {
		return info, nil
	}
	return nil, m.statErr
}
func (m *mockFsUtils) ReadFile(name string) ([]byte, error) {
	if content, ok := m.readFileMap[name]; ok {
		return content, nil
	}
	return nil, m.readFileErr
}
func (m *mockFsUtils) UserHomeDir() (string, error) { return m.homeDir, m.homeDirErr }
func (m *mockFsUtils) Getwd() (string, error) { return m.cwd, m.cwdErr }
func (m *mockFsUtils) LookPath(file string) (string, error) {
	if path, ok := m.lookPathMap[file]; ok {
		return path, nil
	}
	return "", m.lookPathErr
}

func TestDoctorCommand(t *testing.T) {
	// Save original stdout and restore after test
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	defer func() {
		os.Stdout = oldStdout
	}()

	// Test case 1: No config file, otel-cli not found
	// This should result in a failure for MCP config and a warning for otel-cli
	mockUtils1 := &mockFsUtils{
		executable:    "/usr/local/bin/otlp-mcp",
		homeDir:       "/home/testuser",
		cwd:           "/home/testuser/project",
		statMap: map[string]os.FileInfo{
			"/usr/local/bin/otlp-mcp": &mockFileInfo{mode: 0755}, // Executable exists and is executable
		},
		statErr:       os.ErrNotExist, // Simulate no config file for other paths
		lookPathErr:   os.ErrNotExist, // Simulate otel-cli not found
	}

	var buf bytes.Buffer
	outC := make(chan string)
	go func() {
		_, _ = io.Copy(&buf, r)
		outC <- buf.String()
	}()

	err := runDoctorWithUtils("test-version", mockUtils1)
	w.Close()
	out := <-outC

	assert.Error(t, err)
	assert.Contains(t, out, "✗ MCP config not found")
	assert.Contains(t, out, "⚠ Optional: otel-cli not found")
	assert.Contains(t, out, "❌ Found 1 issue(s) that need attention")

	// Test case 2: .gemini/settings.json exists, otel-cli found (mocked)
	// This should result in all checks passing
	buf.Reset()

	// Create a dummy .gemini/settings.json content
	geminiConfigContent := []byte(`{
		"mcpServers": {
			"otlp-mcp": {
				"command": "/usr/local/bin/otlp-mcp",
				"args": ["serve"]
			}
		}
	}`)

	mockUtils2 := &mockFsUtils{
		executable: "/usr/local/bin/otlp-mcp",
		homeDir:    "/home/testuser",
		cwd:        "/home/testuser/project",
		statMap: map[string]os.FileInfo{
			filepath.Join("/home/testuser/project", ".gemini", "settings.json"): &mockFileInfo{mode: 0644},
			"/usr/local/bin/otlp-mcp": &mockFileInfo{mode: 0755},
		},
		readFileMap: map[string][]byte{
			filepath.Join("/home/testuser/project", ".gemini", "settings.json"): geminiConfigContent,
		},
		lookPathMap: map[string]string{
			"otel-cli": "/usr/local/bin/otel-cli",
		},
	}

	r, w, _ = os.Pipe()
	os.Stdout = w
	outC = make(chan string)
	go func() {
		_, _ = io.Copy(&buf, r)
		outC <- buf.String()
	}()

	err = runDoctorWithUtils("test-version", mockUtils2)
	w.Close()
	out = <-outC

	assert.NoError(t, err)
	assert.Contains(t, out, "✓ Gemini CLI config found: ")
	assert.Contains(t, out, "✓ Optional: otel-cli found at /usr/local/bin/otel-cli")
	assert.Contains(t, out, "✅ All checks passed!")
}

// mockFileInfo implements os.FileInfo for testing purposes
type mockFileInfo struct{
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
	sys     interface{}
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return m.sys }
