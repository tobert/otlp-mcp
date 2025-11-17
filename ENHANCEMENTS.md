# Enhancement Plans

Based on feedback from Gemini CLI, here are planned enhancements to improve agent and user experience with otlp-mcp.

## 1. Pre-compiled Binaries (High Priority)

**Goal:** Eliminate Go toolchain requirement for users. Agents can instruct users to download a single binary instead of building from source.

**Implementation:**
- Set up GitHub Actions workflow for releases
- Build binaries for: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
- Create release automation that triggers on git tags (e.g., `v0.3.0`)
- Include checksums (SHA256) for verification
- Update README with download instructions

**Technical approach:**
```yaml
# .github/workflows/release.yml
- Use goreleaser for cross-compilation and release management
- Matrix build: GOOS x GOARCH combinations
- Upload to GitHub Releases
- Generate release notes from jj/git history
```

**README changes:**
```markdown
### Option 1: Download Pre-built Binary (Recommended)

**Linux/macOS:**
```bash
# Download latest release
curl -L https://github.com/tobert/otlp-mcp/releases/latest/download/otlp-mcp-linux-amd64 -o otlp-mcp
chmod +x otlp-mcp
```

**Windows:**
Download from [Releases](https://github.com/tobert/otlp-mcp/releases/latest)
```

**Effort:** ~4 hours (workflow setup + testing + docs)

**Benefits:**
- Agents can guide users through simple download
- No Go installation required
- Faster onboarding
- Professional distribution

---

## 2. Doctor Command (High Priority)

**Goal:** Help agents and users troubleshoot setup issues with a single diagnostic command.

**Implementation:**
- Add `otlp-mcp doctor` subcommand
- Run comprehensive checks and report results
- Exit code 0 = all good, 1 = issues found

**Checks to perform:**
1. **Binary location** - Show where otlp-mcp is installed
2. **MCP config** - Try to find and parse mcp_settings.json
3. **Path validation** - Verify config paths point to actual binary
4. **Port availability** - Check if OTLP port is available (if specified)
5. **Dependencies** - Check for optional tools (otel-cli)
6. **Permissions** - Verify binary is executable

**Command structure:**
```go
// cmd/otlp-mcp/doctor.go
func runDoctor(cmd *cobra.Command, args []string) error {
    checks := []Check{
        checkBinaryLocation,
        checkMCPConfig,
        checkPortAvailability,
        checkDependencies,
    }

    results := runChecks(checks)
    printResults(results)

    if results.HasErrors() {
        return fmt.Errorf("doctor found %d issues", results.ErrorCount())
    }
    return nil
}
```

**Output format:**
```
🔍 otlp-mcp doctor v0.3.0

✓ Binary location: /home/user/otlp-mcp/otlp-mcp
✓ Binary is executable
✓ MCP config found: ~/.config/claude-code/mcp_settings.json
✓ Config path matches binary: /home/user/otlp-mcp/otlp-mcp
✓ Default OTLP port 0 (ephemeral) available
⚠ Optional: otel-cli not found (install with: go install github.com/tobert/otel-cli@latest)

✅ All critical checks passed!
💡 Run 'otlp-mcp serve --verbose' to start the server
```

**Error example:**
```
✓ Binary location: /home/user/otlp-mcp/otlp-mcp
✗ MCP config not found at ~/.config/claude-code/mcp_settings.json

  Create the config file with:
  {
    "mcpServers": {
      "otlp-mcp": {
        "command": "/home/user/otlp-mcp/otlp-mcp",
        "args": ["serve", "--verbose"]
      }
    }
  }

❌ Found 1 issue that needs attention
```

**Effort:** ~6 hours (checks implementation + error messages + testing)

**Benefits:**
- Agents can ask "run otlp-mcp doctor" to diagnose issues
- Actionable error messages with fix suggestions
- Reduces support burden
- Improves user confidence

---

## 3. Richer Query Filters (Medium Priority)

**Goal:** Enable server-side filtering for common queries to reduce data transfer and client-side processing.

**New filter parameters:**

```go
type QueryInput struct {
    // Existing filters
    ServiceName   string   `json:"service_name,omitempty"`
    TraceID       string   `json:"trace_id,omitempty"`
    SpanName      string   `json:"span_name,omitempty"`
    LogSeverity   string   `json:"log_severity,omitempty"`
    MetricNames   []string `json:"metric_names,omitempty"`
    StartSnapshot string   `json:"start_snapshot,omitempty"`
    EndSnapshot   string   `json:"end_snapshot,omitempty"`
    Limit         int      `json:"limit,omitempty"`

    // NEW: Status filtering
    SpanStatus    string   `json:"span_status,omitempty"`    // "STATUS_CODE_OK", "STATUS_CODE_ERROR", "STATUS_CODE_UNSET"
    ErrorsOnly    bool     `json:"errors_only,omitempty"`    // Shortcut for STATUS_CODE_ERROR

    // NEW: Duration filtering (in nanoseconds)
    MinDuration   *uint64  `json:"min_duration_ns,omitempty"` // e.g., 500_000_000 for 500ms
    MaxDuration   *uint64  `json:"max_duration_ns,omitempty"`

    // NEW: Time range filtering (alternative to snapshots)
    StartTime     *time.Time `json:"start_time,omitempty"`
    EndTime       *time.Time `json:"end_time,omitempty"`

    // NEW: Attribute filtering
    HasAttribute  string     `json:"has_attribute,omitempty"`    // e.g., "http.status_code"
    AttributeEq   map[string]string `json:"attribute_equals,omitempty"` // e.g., {"http.status_code": "500"}
}
```

**Example queries:**
```
# Find slow database operations
query(service_name="database", min_duration_ns=500_000_000)

# Find all errors
query(errors_only=true)

# Find HTTP 500 errors
query(attribute_equals={"http.status_code": "500"})

# Find spans with specific attribute
query(has_attribute="db.statement")
```

**Implementation approach:**
1. Extend `storage.QueryFilter` with new fields
2. Update `storage.Query()` to apply filters during iteration
3. Add filter logic in span/log/metric matching
4. Update MCP tool schema and handler

**Effort:** ~8 hours (storage updates + testing + docs)

**Benefits:**
- Faster queries (less data transfer)
- More precise results
- Common debugging patterns built-in
- Reduces agent-side filtering code

---

## 4. Run-and-Compare Tool (Medium Priority)

**Goal:** Single MCP tool that automates the snapshot → run → snapshot → diff workflow for test-driven development.

**New MCP tool:**

```go
type RunAndCompareInput struct {
    Command     string   `json:"command"`              // Command to run (e.g., "go test ./...")
    Args        []string `json:"args,omitempty"`       // Command arguments
    Env         map[string]string `json:"env,omitempty"` // Additional environment variables
    WorkDir     string   `json:"work_dir,omitempty"`   // Working directory (default: cwd)
    SnapshotPrefix string `json:"snapshot_prefix,omitempty"` // Prefix for snapshot names (default: "compare")
}

type RunAndCompareOutput struct {
    Command      string          `json:"command"`           // Full command executed
    ExitCode     int             `json:"exit_code"`         // Command exit code
    Stdout       string          `json:"stdout"`            // Command stdout
    Stderr       string          `json:"stderr"`            // Command stderr
    Duration     string          `json:"duration"`          // Command duration
    BeforeSnapshot string        `json:"before_snapshot"`   // Name of before snapshot
    AfterSnapshot  string        `json:"after_snapshot"`    // Name of after snapshot
    Telemetry    TelemetryDiff   `json:"telemetry"`         // Diff of telemetry
}

type TelemetryDiff struct {
    NewTraces   int              `json:"new_traces"`        // Traces that appeared
    NewLogs     int              `json:"new_logs"`          // Logs that appeared
    NewMetrics  int              `json:"new_metrics"`       // Metrics that appeared
    ErrorLogs   []LogSummary     `json:"error_logs"`        // ERROR level logs
    SlowSpans   []TraceSummary   `json:"slow_spans"`        // Spans over threshold
    Summary     string           `json:"summary"`           // Human-readable summary
}
```

**Workflow:**
1. Create "before" snapshot: `{prefix}-before-{timestamp}`
2. Get current OTLP endpoint
3. Set `OTEL_EXPORTER_OTLP_ENDPOINT` in environment
4. Execute command with timeout
5. Create "after" snapshot: `{prefix}-after-{timestamp}`
6. Get snapshot diff
7. Analyze telemetry (find errors, slow spans, etc.)
8. Return comprehensive results

**Example usage:**
```
You: Run my tests and show me what telemetry they produce

Agent: [calls run_and_compare]
       Command: go test ./...
       Exit code: 0 (passed)
       Duration: 2.3s

       Telemetry captured:
       - 47 new traces (15 test spans, 32 database operations)
       - 12 new logs (all INFO level)
       - 0 errors

       Slowest operations:
       - TestDatabaseIntegration: 1.8s
       - TestAPIEndpoint: 450ms
```

**Safety considerations:**
- Timeout parameter (default: 60s, max: 600s)
- Validate command against allowlist? (security consideration)
- Capture stdout/stderr with size limits
- Auto-cleanup old snapshots from previous runs

**Effort:** ~10 hours (command execution + snapshot management + analysis + testing)

**Benefits:**
- Single tool replaces multi-step workflow
- Perfect for TDD and test observation
- Automatic error detection and reporting
- Reduces agent complexity

---

## Implementation Priority

**Phase 1 (v0.3.0): Essential QoL**
1. Pre-compiled binaries (4h) - Removes biggest onboarding friction
2. Doctor command (6h) - Enables self-service troubleshooting

**Phase 2 (v0.4.0): Query Enhancements**
3. Richer query filters (8h) - Improves agent efficiency

**Phase 3 (v0.5.0): Advanced Workflows**
4. Run-and-compare tool (10h) - Enables powerful testing workflows

**Total effort:** ~28 hours across 3 releases

---

## Open Questions

1. **Security for run_and_compare:** Should we restrict which commands can be executed? Or trust that agents/users won't run malicious commands?
2. **Snapshot retention:** Should we auto-delete old run-and-compare snapshots? How many to keep?
3. **Query filter syntax:** Should we support more complex expressions (AND/OR logic)?
4. **Cross-platform doctor:** How to handle platform-specific checks (e.g., Windows paths)?

---

## Feedback Integration

These plans are based on Gemini's feedback that:
- ✅ The core concept and documentation are excellent
- ⚠️ Setup process has friction points
- 💡 More sophisticated queries would reduce agent-side work
- 🎯 Higher-level workflows would be powerful

All enhancements maintain the core philosophy: **enable agents to observe and iterate on code execution.**
