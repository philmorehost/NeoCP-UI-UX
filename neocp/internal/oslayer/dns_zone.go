package oslayer

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"neocp/internal/core"
)

// GenerateBind9ZoneFile compiles BIND9 zone records and writes to dns_zones/<domain>.db
func GenerateBind9ZoneFile(domainName string, records []core.DNSRecord, workspaceDir string) (string, error) {
	dnsDir := filepath.Join(workspaceDir, "dns_zones")
	err := os.MkdirAll(dnsDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create dns_zones directory: %v", err)
	}

	serial := time.Now().Format("20060102") + "01"

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("; BIND9 Zone File for %s (Compiled by NeoCP Core Engine)\n", domainName))
	sb.WriteString(fmt.Sprintf("; Compiled At: %s\n\n", time.Now().Format(time.RFC1123)))
	sb.WriteString("$TTL 86400\n")
	sb.WriteString("@   IN  SOA  ns1.neocp.io. admin.neocp.io. (\n")
	sb.WriteString(fmt.Sprintf("              %s ; Serial\n", serial))
	sb.WriteString("              3600       ; Refresh\n")
	sb.WriteString("              1800       ; Retry\n")
	sb.WriteString("              604800     ; Expire\n")
	sb.WriteString("              86400 )    ; Minimum TTL\n\n")

	// Standard Nameservers
	sb.WriteString("; Name Server Registry\n")
	sb.WriteString("@   IN  NS  ns1.neocp.io.\n")
	sb.WriteString("@   IN  NS  ns2.neocp.io.\n\n")

	sb.WriteString("; Active Resource Records\n")
	for _, r := range records {
		name := r.Name
		if name == "" {
			name = "@"
		}
		ttlStr := ""
		if r.TTL > 0 {
			ttlStr = fmt.Sprintf("%d", r.TTL)
		} else {
			ttlStr = "86400"
		}

		switch strings.ToUpper(r.Type) {
		case "MX":
			priority := r.Priority
			if priority == 0 {
				priority = 10
			}
			sb.WriteString(fmt.Sprintf("%-16s %-6s IN  MX  %-3d %s\n", name, ttlStr, priority, r.Value))
		case "SRV":
			priority := r.Priority
			if priority == 0 {
				priority = 10
			}
			// SRV requires priority, weight, port, target
			// Value format could be: "10 5060 sip.domain.com"
			sb.WriteString(fmt.Sprintf("%-16s %-6s IN  SRV %-3d %s\n", name, ttlStr, priority, r.Value))
		default:
			// A, AAAA, CNAME, TXT
			val := r.Value
			if strings.ToUpper(r.Type) == "TXT" {
				// wrap text in quotes if not already wrapped
				if !strings.HasPrefix(val, "\"") {
					val = "\"" + val + "\""
				}
			}
			sb.WriteString(fmt.Sprintf("%-16s %-6s IN  %-5s %s\n", name, ttlStr, r.Type, val))
		}
	}

	content := sb.String()
	filePath := filepath.Join(dnsDir, domainName+".db")
	err = ioutil.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write BIND9 zone file: %v", err)
	}

	return content, nil
}

// RemoveBind9ZoneFile deletes a BIND9 zone configuration
func RemoveBind9ZoneFile(domainName string, workspaceDir string) error {
	filePath := filepath.Join(workspaceDir, "dns_zones", domainName+".db")
	if _, err := os.Stat(filePath); err == nil {
		return os.Remove(filePath)
	}
	return nil
}
