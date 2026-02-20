package utils

import (
	"fmt"
	"net"
)

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
