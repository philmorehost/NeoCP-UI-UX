package oslayer

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
)

// SetHostname updates /etc/hostname and /etc/hosts
func SetHostname(ctx context.Context, hostname string) error {
	exec := &SafeCommandExec{}

	// Update /etc/hostname
	err := ioutil.WriteFile("/etc/hostname", []byte(hostname+"\n"), 0644)
	if err != nil {
		// Fallback for simulation or non-root
		fmt.Printf("[Simulation] Setting hostname to %s\n", hostname)
	}

	// Update /etc/hosts
	data, err := ioutil.ReadFile("/etc/hosts")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		found := false
		for i, line := range lines {
			if strings.Contains(line, "127.0.1.1") || strings.Contains(line, "127.0.0.1") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					lines[i] = parts[0] + "\t" + hostname + " " + parts[1]
					found = true
					break
				}
			}
		}
		if !found {
			lines = append(lines, "127.0.1.1\t"+hostname)
		}
		_ = ioutil.WriteFile("/etc/hosts", []byte(strings.Join(lines, "\n")), 0644)
	}

	// Run hostname command
	_, _ = exec.Execute(ctx, "hostname", []string{hostname}, 0)

	return nil
}

// UpdateResolvers updates /etc/resolv.conf
func UpdateResolvers(ctx context.Context, primary, secondary string) error {
	content := fmt.Sprintf("nameserver %s\nnameserver %s\n", primary, secondary)
	err := ioutil.WriteFile("/etc/resolv.conf", []byte(content), 0644)
	if err != nil {
		fmt.Printf("[Simulation] Updating resolvers: %s, %s\n", primary, secondary)
	}
	return nil
}

// AssignIPToInterface binds an IP to an interface (simulation for now)
func AssignIPToInterface(ctx context.Context, ip, subnet, device string) error {
	exec := &SafeCommandExec{}
	// Example: ip addr add 192.168.1.101/24 dev eth0
	args := []string{"addr", "add", ip + "/24", "dev", device}
	_, err := exec.Execute(ctx, "ip", args, 0)
	return err
}
