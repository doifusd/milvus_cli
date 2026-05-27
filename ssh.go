package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// SSHClient wraps the established SSH connection.
type SSHClient struct {
	client *ssh.Client
}

// ConnectSSH establishes a connection to the SSH host.
func ConnectSSH(cfg *Config) (*SSHClient, error) {
	if cfg.SSHHost == "" {
		return nil, errors.New("SSH host is empty")
	}

	var authMethods []ssh.AuthMethod

	// 1. Try private key auth first if key path is specified
	if cfg.SSHKeyPath != "" {
		fmt.Printf("[DEBUG] Reading SSH key from path: %s\n", cfg.SSHKeyPath)
		keyBytes, err := os.ReadFile(cfg.SSHKeyPath)
		if err == nil {
			var signer ssh.Signer
			if cfg.SSHKeyPass != "" {
				fmt.Println("[DEBUG] Parsing key with passphrase...")
				signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(cfg.SSHKeyPass))
			} else {
				fmt.Println("[DEBUG] Parsing key without passphrase...")
				signer, err = ssh.ParsePrivateKey(keyBytes)
				if err != nil {
					// Check if key requires passphrase
					if _, ok := err.(*ssh.PassphraseMissingError); ok {
						fmt.Println("[DEBUG] Key requires passphrase. Prompting user...")
						// Prompt for passphrase securely if we are running in a terminal
						fmt.Printf("Enter passphrase for SSH key (%s): ", cfg.SSHKeyPath)
						passBytes, readErr := term.ReadPassword(int(os.Stdin.Fd()))
						fmt.Println()
						if readErr == nil && len(passBytes) > 0 {
							cfg.SSHKeyPass = string(passBytes)
							signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, passBytes)
						}
					}
				}
			}

			if err == nil {
				fmt.Println("[DEBUG] Key parsed successfully! Appending PublicKeys auth method.")
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			} else {
				fmt.Printf("⚠️  SSH Key parse error: %v (falling back to password auth if available)\n", err)
			}
		} else {
			fmt.Printf("[DEBUG] Failed to read key file: %v\n", err)
			if !os.IsNotExist(err) {
				fmt.Printf("⚠️  SSH Key read error: %v\n", err)
			}
		}
	}

	// 2. Try password auth if provided or prompt if we have no valid authentication methods
	if cfg.SSHPassword != "" {
		fmt.Println("[DEBUG] Appending Password auth method.")
		authMethods = append(authMethods, ssh.Password(cfg.SSHPassword))
	}

	// If no auth methods available, prompt for password
	if len(authMethods) == 0 {
		fmt.Println("[DEBUG] No auth methods configured. Prompting for SSH password...")
		fmt.Printf("Enter SSH password for %s@%s: ", cfg.SSHUser, cfg.SSHHost)
		passBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err == nil && len(passBytes) > 0 {
			cfg.SSHPassword = string(passBytes)
			authMethods = append(authMethods, ssh.Password(cfg.SSHPassword))
		} else {
			return nil, errors.New("no SSH authentication methods provided")
		}
	}

	clientConfig := &ssh.ClientConfig{
		User:            cfg.SSHUser,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // For convenience in CLI tools, can be configured
		Timeout:         15 * time.Second,
	}

	host := cfg.SSHHost
	if !strings.Contains(host, ":") {
		host = host + ":22"
	}

	fmt.Printf("[DEBUG] Dialing SSH server: tcp://%s (User: %s, Auth methods: %d)\n", host, clientConfig.User, len(clientConfig.Auth))
	client, err := ssh.Dial("tcp", host, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH server: %w", err)
	}

	return &SSHClient{client: client}, nil
}

// DialContext creates a net.Conn to the target address through the SSH connection.
// This matches the signature expected by gRPC's grpc.WithContextDialer.
func (s *SSHClient) DialContext(ctx context.Context, addr string) (net.Conn, error) {
	// A helper struct or channel can be used to handle context cancellation.
	// Since s.client.Dial does not accept a context natively, we dial asynchronously
	// and handle context timeout/cancellation.
	
	type dialResult struct {
		conn net.Conn
		err  error
	}

	ch := make(chan dialResult, 1)

	go func() {
		conn, err := s.client.Dial("tcp", addr)
		ch <- dialResult{conn: conn, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		return res.conn, res.err
	}
}

// Close closes the underlying SSH connection.
func (s *SSHClient) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
