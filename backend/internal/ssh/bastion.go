package ssh

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	ErrSSHConfigMissing = errors.New("ssh bastion configuration is incomplete")
	ErrTunnelClosed     = errors.New("ssh tunnel is closed")
)

// BastionConfig holds connection details for an SSH jump host
type BastionConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
	Timeout    int    `json:"timeout_seconds"`
}

// TunnelPool manages reused SSH client sessions
type TunnelPool struct {
	mu      sync.RWMutex
	clients map[string]*pooledClient
}

type pooledClient struct {
	client    *ssh.Client
	lastUsed  time.Time
	configKey string
}

// NewTunnelPool creates a new SSH tunnel pool
func NewTunnelPool() *TunnelPool {
	pool := &TunnelPool{
		clients: make(map[string]*pooledClient),
	}
	// Background cleanup of idle SSH connections (after 10 minutes)
	go pool.reapIdleClients()
	return pool
}

// DialContext connects to remote database host through the SSH jump host
func (p *TunnelPool) DialContext(ctx context.Context, bastion BastionConfig, targetHost string, targetPort int) (net.Conn, error) {
	if bastion.Host == "" || bastion.User == "" {
		return nil, ErrSSHConfigMissing
	}
	if bastion.Port <= 0 {
		bastion.Port = 22
	}

	client, err := p.getOrCreateClient(ctx, bastion)
	if err != nil {
		return nil, fmt.Errorf("failed to establish ssh jump client: %w", err)
	}

	targetAddr := fmt.Sprintf("%s:%d", targetHost, targetPort)
	conn, err := client.DialContext(ctx, "tcp", targetAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial remote target %s via ssh: %w", targetAddr, err)
	}

	return conn, nil
}

func (p *TunnelPool) getOrCreateClient(ctx context.Context, cfg BastionConfig) (*ssh.Client, error) {
	key := p.configHash(cfg)

	p.mu.RLock()
	pooled, exists := p.clients[key]
	p.mu.RUnlock()

	if exists && pooled.client != nil {
		// Verify if alive by sending a global keepalive request
		_, _, err := pooled.client.SendRequest("keepalive@muara.bi", true, nil)
		if err == nil {
			p.mu.Lock()
			pooled.lastUsed = time.Now()
			p.mu.Unlock()
			return pooled.client, nil
		}
		// If dead, remove from pool
		_ = pooled.client.Close()
		p.mu.Lock()
		delete(p.clients, key)
		p.mu.Unlock()
	}

	// Create new client
	sshConfig, err := buildSSHClientConfig(cfg)
	if err != nil {
		return nil, err
	}

	bastionAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	d := net.Dialer{Timeout: time.Duration(cfg.Timeout) * time.Second}
	if cfg.Timeout <= 0 {
		d.Timeout = 15 * time.Second
	}

	netConn, err := d.DialContext(ctx, "tcp", bastionAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial bastion host %s: %w", bastionAddr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(netConn, bastionAddr, sshConfig)
	if err != nil {
		_ = netConn.Close()
		return nil, fmt.Errorf("ssh handshake failed with bastion: %w", err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)

	p.mu.Lock()
	p.clients[key] = &pooledClient{
		client:    client,
		lastUsed:  time.Now(),
		configKey: key,
	}
	p.mu.Unlock()

	return client, nil
}

func (p *TunnelPool) configHash(cfg BastionConfig) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%d:%s:%s", cfg.Host, cfg.Port, cfg.User, cfg.Password)))
	if cfg.PrivateKey != "" {
		h.Write([]byte(cfg.PrivateKey))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (p *TunnelPool) reapIdleClients() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		now := time.Now()
		for k, item := range p.clients {
			if now.Sub(item.lastUsed) > 10*time.Minute {
				_ = item.client.Close()
				delete(p.clients, k)
			}
		}
		p.mu.Unlock()
	}
}

// CloseAll terminates all pooled SSH tunnels
func (p *TunnelPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, item := range p.clients {
		if item.client != nil {
			_ = item.client.Close()
		}
	}
	p.clients = make(map[string]*pooledClient)
}

func buildSSHClientConfig(cfg BastionConfig) (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	if cfg.PrivateKey != "" {
		var signer ssh.Signer
		var err error
		if cfg.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(cfg.PrivateKey), []byte(cfg.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(cfg.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse ssh private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}

	if len(authMethods) == 0 {
		return nil, errors.New("neither private key nor password provided for ssh authentication")
	}

	timeout := 15 * time.Second
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}

	return &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // In production, known_hosts can be injected
		Timeout:         timeout,
	}, nil
}
