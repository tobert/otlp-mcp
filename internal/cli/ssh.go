package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/sshtransport"
	"github.com/tobert/otlp-mcp/internal/mcpserver"
)

// runSSHTransport starts the MCP server using SSH transport.
// It loads or generates a host key, parses authorized keys, installs
// authorization middleware, and starts the SSH handler.
func runSSHTransport(ctx context.Context, cfg *Config, mcpServer *mcpserver.Server, otlpErrChan chan error) error {
	// 1. Load or generate host key.
	hostKeyPath := cfg.SSHHostKeyFile
	if hostKeyPath == "" {
		// Default to ~/.config/otlp-mcp/host_key
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("getting home directory: %w", err)
		}
		configDir := filepath.Join(home, ".config", "otlp-mcp")
		if err := os.MkdirAll(configDir, 0700); err != nil {
			return fmt.Errorf("creating config directory: %w", err)
		}
		hostKeyPath = filepath.Join(configDir, "ssh_host_key")
	}

	hostKey, err := sshtransport.LoadOrGenerateHostKey(hostKeyPath)
	if err != nil {
		return fmt.Errorf("loading host key: %w", err)
	}
	log.Printf("🔑 SSH host key: %s\n", hostKeyPath)

	// 2. Parse authorized keys file.
	if cfg.SSHAuthorizedKeys == "" {
		return fmt.Errorf("--ssh-authorized-keys is required for SSH transport")
	}

	authorizedKeys, err := sshtransport.ParseAuthorizedKeysFile(cfg.SSHAuthorizedKeys)
	if err != nil {
		return fmt.Errorf("parsing authorized keys: %w", err)
	}
	log.Printf("🔑 SSH authorized keys: %s\n", cfg.SSHAuthorizedKeys)

	// 3. Install authorization middleware on the MCP server.
	mcpServer.MCPServer().AddReceivingMiddleware(sshtransport.AuthorizationMiddleware())

	// 4. Create SSH handler.
	handler := sshtransport.NewSSHHandler(
		func() *mcp.Server { return mcpServer.MCPServer() },
		&sshtransport.SSHHandlerOptions{
			HostKey:        hostKey,
			AuthorizedKeys: authorizedKeys,
			Subsystems:     []string{"mcp"},
		},
	)

	// 5. Start SSH server in background.
	addr := fmt.Sprintf("%s:%d", cfg.SSHHost, cfg.SSHPort)
	sshErrChan := make(chan error, 1)
	go func() {
		sshErrChan <- handler.ListenAndServe(ctx, addr)
	}()

	// 6. Wait for shutdown or errors.
	select {
	case <-ctx.Done():
		mcpServer.Shutdown()
		return handler.Close()

	case err := <-sshErrChan:
		if err != nil {
			return fmt.Errorf("SSH server error: %w", err)
		}
		return nil

	case err := <-otlpErrChan:
		if err != nil {
			return fmt.Errorf("OTLP receiver error: %w", err)
		}
		return nil
	}
}
