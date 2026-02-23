package utils

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// GetGlobalPortsFile returns the path to the global ports tracking file
func GetGlobalPortsFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	gravaDir := filepath.Join(home, ".grava")
	if err := os.MkdirAll(gravaDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(gravaDir, "ports.json"), nil
}

// LoadUsedPorts returns a map of project paths to used ports.
func LoadUsedPorts() (map[string]int, error) {
	portsFile, err := GetGlobalPortsFile()
	if err != nil {
		return nil, err
	}

	b, err := os.ReadFile(portsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]int), nil
		}
		return nil, err
	}

	var ports map[string]int
	if err := json.Unmarshal(b, &ports); err != nil {
		return nil, err
	}

	return ports, nil
}

// SaveUsedPort saves the project path and port to the global tracking file.
func SaveUsedPort(projectPath string, port int) error {
	ports, err := LoadUsedPorts()
	if err != nil {
		return err
	}

	ports[projectPath] = port

	portsFile, err := GetGlobalPortsFile()
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(ports, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(portsFile, b, 0644)
}

// AllocatePort finds the next available port that is not already tracked in the ports.json file.
func AllocatePort(projectPath string, startPort int) (int, error) {
	ports, err := LoadUsedPorts()
	if err != nil {
		return -1, fmt.Errorf("failed to load used ports: %w", err)
	}

	// Check if this project already has an assigned port
	if port, exists := ports[projectPath]; exists {
		// We trust the assigned port
		return port, nil
	}

	usedPorts := make(map[int]bool)
	for _, p := range ports {
		usedPorts[p] = true
	}

	// Find the next available port not in usedPorts
	for port := startPort; port < startPort+1000; port++ {
		if usedPorts[port] {
			continue // Skip already assigned port
		}

		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			// We found an available port
			if err := SaveUsedPort(projectPath, port); err != nil {
				return -1, fmt.Errorf("failed to save allocated port: %w", err)
			}
			return port, nil
		}
	}

	return -1, fmt.Errorf("could not find an available port starting from %d", startPort)
}

// FindAvailablePort scans for an available TCP port starting from the given port.
// It returns the first available port found, or -1 if none are found within 100 attempts.
func FindAvailablePort(start int) int {
	for port := start; port < start+100; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	return -1
}
