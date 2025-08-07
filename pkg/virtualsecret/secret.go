package virtualsecret

import (
	"crypto/rand"
	"fmt"
)

// VirtualMCPSecret stores the generated secret for virtual MCP access
var virtualMCPSecret string

// init generates the virtual MCP secret during package initialization
func init() {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	virtualMCPSecret = fmt.Sprintf("%x", bytes)
}

// GetVirtualMCPSecret returns the current virtual MCP secret
func GetVirtualMCPSecret() string {
	return virtualMCPSecret
}
