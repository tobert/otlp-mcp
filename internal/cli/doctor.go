package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/urfave/cli/v3"
)

// DoctorCommand returns the CLI command definition for the 'doctor' subcommand.
// This command runs diagnostic checks to verify otlp-mcp is properly configured.
func DoctorCommand(version string) *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "Diagnose common setup and configuration issues",
		Description: `Run comprehensive checks to verify otlp-mcp is properly configured.

This command checks:
  - Binary location and permissions
  - MCP configuration file (mcp_settings.json)
  - Path validation in configuration
  - Optional dependencies (otel-cli)

Exit codes:
  0 - All critical checks passed
  1 - One or more issues found`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runDoctor(version)
		},
	}
}

type checkResult struct {
	Name       string
	Status     string // "pass", "warn", "fail"
	Message    string
	Suggestion string
	IsCritical bool
}

func runDoctor(version string) error {
	fmt.Printf("🔍 otlp-mcp doctor v%s\n\n", version)

	checks := []func() checkResult{
		checkBinaryLocation,
		checkBinaryExecutable,
		checkMCPConfig,
		checkOtelCLI,
	}

	results := make([]checkResult, 0, len(checks))
	for _, check := range checks {
		result := check()
		results = append(results, result)
		printCheckResult(result)
	}

	fmt.Println()
	summary := summarizeResults(results)
	printSummary(summary)

	if summary.FailCount > 0 {
		return fmt.Errorf("found %d issues that need attention", summary.FailCount)
	}

	return nil
}

func printCheckResult(result checkResult) {
	var icon string
	switch result.Status {
	case "pass":
		icon = "✓"
	case "warn":
		icon = "⚠"
	case "fail":
		icon = "✗"
	}

	fmt.Printf("%s %s\n", icon, result.Message)

	if result.Suggestion != "" {
		fmt.Printf("  %s\n", result.Suggestion)
	}
}

type resultSummary struct {
	PassCount int
	WarnCount int
	FailCount int
}

func summarizeResults(results []checkResult) resultSummary {
	var summary resultSummary
	for _, r := range results {
		switch r.Status {
		case "pass":
			summary.PassCount++
		case "warn":
			summary.WarnCount++
		case "fail":
			summary.FailCount++
		}
	}
	return summary
}

func printSummary(summary resultSummary) {
	if summary.FailCount > 0 {
		fmt.Printf("❌ Found %d issue(s) that need attention\n", summary.FailCount)
		if summary.WarnCount > 0 {
			fmt.Printf("⚠️  %d warning(s)\n", summary.WarnCount)
		}
	} else if summary.WarnCount > 0 {
		fmt.Printf("✅ All critical checks passed!\n")
		fmt.Printf("⚠️  %d optional warning(s)\n", summary.WarnCount)
		fmt.Printf("💡 Run 'otlp-mcp serve --verbose' to start the server\n")
	} else {
		fmt.Printf("✅ All checks passed!\n")
		fmt.Printf("💡 Run 'otlp-mcp serve --verbose' to start the server\n")
	}
}

// Check 1: Binary location
func checkBinaryLocation() checkResult {
	executable, err := os.Executable()
	if err != nil {
		return checkResult{
			Name:       "binary_location",
			Status:     "fail",
			Message:    "Could not determine binary location",
			Suggestion: fmt.Sprintf("Error: %v", err),
			IsCritical: true,
		}
	}

	absPath, err := filepath.Abs(executable)
	if err != nil {
		absPath = executable
	}

	return checkResult{
		Name:       "binary_location",
		Status:     "pass",
		Message:    fmt.Sprintf("Binary location: %s", absPath),
		IsCritical: false,
	}
}

// Check 2: Binary executable
func checkBinaryExecutable() checkResult {
	executable, err := os.Executable()
	if err != nil {
		return checkResult{
			Name:       "binary_executable",
			Status:     "fail",
			Message:    "Could not check if binary is executable",
			IsCritical: true,
		}
	}

	info, err := os.Stat(executable)
	if err != nil {
		return checkResult{
			Name:       "binary_executable",
			Status:     "fail",
			Message:    "Could not stat binary",
			Suggestion: fmt.Sprintf("Error: %v", err),
			IsCritical: true,
		}
	}

	mode := info.Mode()
	if mode&0111 == 0 {
		return checkResult{
			Name:       "binary_executable",
			Status:     "fail",
			Message:    "Binary is not executable",
			Suggestion: fmt.Sprintf("Run: chmod +x %s", executable),
			IsCritical: true,
		}
	}

	return checkResult{
		Name:       "binary_executable",
		Status:     "pass",
		Message:    "Binary is executable",
		IsCritical: false,
	}
}

// Check 3: MCP configuration
func checkMCPConfig() checkResult {
	configPath := getMCPConfigPath()
	allPaths := getMCPConfigPaths()

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		executable, _ := os.Executable()
		absPath, _ := filepath.Abs(executable)

		// Build list of possible locations for the suggestion
		locationsList := ""
		for _, p := range allPaths {
			locationsList += fmt.Sprintf("  - %s\n", p)
		}

		suggestion := fmt.Sprintf(`MCP config not found. Checked:
%s
  For Claude Code, create at: %s
  For other MCP agents, use their config location

  Example config:
  {
    "mcpServers": {
      "otlp-mcp": {
        "command": "%s",
        "args": ["serve", "--verbose"]
      }
    }
  }`, locationsList, allPaths[0], absPath)

		return checkResult{
			Name:       "mcp_config",
			Status:     "fail",
			Message:    "MCP config not found",
			Suggestion: suggestion,
			IsCritical: true,
		}
	}

	// Try to parse the JSON
	data, err := os.ReadFile(configPath)
	if err != nil {
		return checkResult{
			Name:       "mcp_config",
			Status:     "fail",
			Message:    "Could not read MCP config",
			Suggestion: fmt.Sprintf("Error reading %s: %v", configPath, err),
			IsCritical: true,
		}
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return checkResult{
			Name:       "mcp_config",
			Status:     "fail",
			Message:    "MCP config is not valid JSON",
			Suggestion: fmt.Sprintf("Error parsing %s: %v", configPath, err),
			IsCritical: true,
		}
	}

	// Detect which agent by config path
	agentName := "MCP agent"
	if strings.Contains(configPath, "claude-code") || strings.Contains(configPath, ".claude") {
		agentName = "Claude Code"
	} else if strings.Contains(configPath, ".gemini") {
		agentName = "Gemini CLI"
	}

	// Check for otlp-mcp entry
	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		return checkResult{
			Name:       "mcp_config",
			Status:     "warn",
			Message:    fmt.Sprintf("%s config found: %s", agentName, configPath),
			Suggestion: "Config does not contain 'mcpServers' section",
			IsCritical: false,
		}
	}

	otlpMcp, ok := mcpServers["otlp-mcp"].(map[string]interface{})
	if !ok {
		return checkResult{
			Name:       "mcp_config",
			Status:     "warn",
			Message:    fmt.Sprintf("%s config found: %s", agentName, configPath),
			Suggestion: "Config does not contain 'otlp-mcp' server entry - add otlp-mcp to use this tool",
			IsCritical: false,
		}
	}

	// Check if command path matches current binary
	configuredCommand, _ := otlpMcp["command"].(string)
	executable, _ := os.Executable()
	absExecutable, _ := filepath.Abs(executable)

	if configuredCommand != "" && configuredCommand != absExecutable {
		return checkResult{
			Name:    "mcp_config",
			Status:  "warn",
			Message: fmt.Sprintf("MCP config found: %s", configPath),
			Suggestion: fmt.Sprintf("Config path (%s) differs from current binary (%s)\n  Update config to use current binary if needed",
				configuredCommand, absExecutable),
			IsCritical: false,
		}
	}

	return checkResult{
		Name:       "mcp_config",
		Status:     "pass",
		Message:    fmt.Sprintf("%s config found: %s", agentName, configPath),
		IsCritical: false,
	}
}

// Check 4: otel-cli availability
func checkOtelCLI() checkResult {
	// Try to find otel-cli in PATH
	_, err := os.Stat("/usr/local/bin/otel-cli")
	if err == nil {
		return checkResult{
			Name:    "otel_cli",
			Status:  "pass",
			Message: "Optional: otel-cli found at /usr/local/bin/otel-cli",
		}
	}

	// Check ~/go/bin
	homeDir, _ := os.UserHomeDir()
	goPath := filepath.Join(homeDir, "go", "bin", "otel-cli")
	_, err = os.Stat(goPath)
	if err == nil {
		return checkResult{
			Name:    "otel_cli",
			Status:  "pass",
			Message: fmt.Sprintf("Optional: otel-cli found at %s", goPath),
		}
	}

	// Check ~/src/otel-cli (user's custom location)
	srcPath := filepath.Join(homeDir, "src", "otel-cli", "otel-cli")
	_, err = os.Stat(srcPath)
	if err == nil {
		return checkResult{
			Name:    "otel_cli",
			Status:  "pass",
			Message: fmt.Sprintf("Optional: otel-cli found at %s", srcPath),
		}
	}

	return checkResult{
		Name:    "otel_cli",
		Status:  "warn",
		Message: "Optional: otel-cli not found",
		Suggestion: `otel-cli is useful for testing but not required.
  Install with: go install github.com/tobert/otel-cli@latest`,
		IsCritical: false,
	}
}

// getMCPConfigPaths returns possible MCP config file paths for various agents
func getMCPConfigPaths() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	cwd, _ := os.Getwd()

	var paths []string

	// Check project-level configs first (more specific)
	if cwd != "" {
		paths = append(paths,
			filepath.Join(cwd, ".gemini", "settings.json"),  // Gemini CLI (per-project)
			filepath.Join(cwd, ".claude", "settings.json"),   // Claude (if per-project exists)
		)
	}

	// Then check global configs
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(homeDir, "AppData", "Roaming")
		}
		paths = append(paths, filepath.Join(appData, "Claude Code", "mcp_settings.json"))
	case "darwin":
		paths = append(paths, filepath.Join(homeDir, ".config", "claude-code", "mcp_settings.json"))
	default: // linux and others
		paths = append(paths, filepath.Join(homeDir, ".config", "claude-code", "mcp_settings.json"))
	}

	return paths
}

// getMCPConfigPath returns the first existing MCP config file path
func getMCPConfigPath() string {
	paths := getMCPConfigPaths()
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	// Return first path as default for error messages
	if len(paths) > 0 {
		return paths[0]
	}
	return ""
}
