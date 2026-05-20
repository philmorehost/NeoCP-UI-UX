package oslayer

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"neocp/internal/core"
)

// GenerateNginxConfig builds a fully functional Nginx Virtual Host config file 
func GenerateNginxConfig(domainName string, owner string, phpVersion string, gzipEnabled bool, brotliEnabled bool, sslActive bool, workspaceDir string) (string, error) {
	nginxVHostsDir := filepath.Join(workspaceDir, "nginx_vhosts")
	err := os.MkdirAll(nginxVHostsDir, 0755)
	if err != nil {
		return "", err
	}

	publicHtmlPath := filepath.Join(workspaceDir, "sandbox", owner, "public_html", domainName)
	// Replace Windows path slashes to conform to standard Nginx configuration paths
	linuxCompatPath := filepath.ToSlash(publicHtmlPath)

	gzipConfig := "# Gzip is disabled"
	if gzipEnabled {
		gzipConfig = `gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml;
    gzip_min_length 1000;`
	}

	brotliConfig := "# Brotli is disabled"
	if brotliEnabled {
		brotliConfig = `brotli on;
    brotli_types text/plain text/css application/json application/javascript text/xml;
    brotli_comp_level 4;`
	}

	sslConfig := ""
	listenPort := "80"
	if sslActive {
		listenPort = "443 ssl http2"
		sslConfig = fmt.Sprintf(`
    ssl_certificate %s/cert.pem;
    ssl_certificate_key %s/key.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;`, filepath.ToSlash(workspaceDir), filepath.ToSlash(workspaceDir))
	}

	// Fetch WAF Policy from database
	db := core.GetDB()
	doms := db.GetDomains(owner, true)
	var waf core.WAFPolicy
	for _, d := range doms {
		if d.DomainName == domainName {
			waf = d.WAFPolicy
			break
		}
	}

	var wafLines []string
	if waf.XSSBlock {
		wafLines = append(wafLines, "    add_header X-XSS-Protection \"1; mode=block\" always;")
		wafLines = append(wafLines, "    add_header Content-Security-Policy \"default-src 'self' http: https: data: blob: 'unsafe-inline'\" always;")
	}
	if waf.CSRFHeader {
		wafLines = append(wafLines, "    add_header X-Frame-Options \"SAMEORIGIN\" always;")
		wafLines = append(wafLines, "    add_header X-Content-Type-Options \"nosniff\" always;")
		wafLines = append(wafLines, "    add_header Referrer-Policy \"strict-origin-when-cross-origin\" always;")
	}
	if waf.SQLiShield {
		wafLines = append(wafLines, "    if ($query_string ~* \"union\\s+select|cast\\(|drop\\s+table|select\\s+.*\\s+from|insert\\s+into\") { return 403; }")
		wafLines = append(wafLines, "    if ($request_uri ~* \"union\\s+select|cast\\(|drop\\s+table|select\\s+.*\\s+from|insert\\s+into\") { return 403; }")
	}
	if waf.LFIShield {
		wafLines = append(wafLines, "    if ($query_string ~* \"\\.\\./|/etc/passwd|/win\\.ini|boot\\.ini|\\.\\.%2f|\\.\\.%2F\") { return 403; }")
		wafLines = append(wafLines, "    if ($request_uri ~* \"\\.\\./|/etc/passwd|/win\\.ini|boot\\.ini|\\.\\.%2f|\\.\\.%2F\") { return 403; }")
	}

	wafConfig := "# OWASP WAF is disabled"
	if len(wafLines) > 0 {
		wafConfig = "# Active OWASP WAF Protection\n" + strings.Join(wafLines, "\n")
	}

	configContent := fmt.Sprintf(`# NeoCP Professional Auto-Generated Config for %s
server {
    listen %s;
    server_name %s www.%s;

    root "%s";
    index index.php index.html index.htm;

    %s

    %s

    %s

    %s

    location / {
        try_files $uri $uri/ /index.php?$args;
    }

    # FastCGI proxy to local PHP-FPM socket mapped to PHP version %s
    location ~ \.php$ {
        include fastcgi_params;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        fastcgi_pass 127.0.0.1:90%s; # PHP version mapped TCP port
        fastcgi_index index.php;
    }

    location ~ /\.ht {
        deny all;
    }

    access_log "%s/%s.access.log";
    error_log "%s/%s.error.log" warn;
}
`, domainName, listenPort, domainName, domainName, linuxCompatPath, gzipConfig, brotliConfig, sslConfig, wafConfig, phpVersion, getPHPPortSuffix(phpVersion), filepath.ToSlash(nginxVHostsDir), domainName, filepath.ToSlash(nginxVHostsDir), domainName)

	confFile := filepath.Join(nginxVHostsDir, fmt.Sprintf("%s.conf", domainName))
	err = ioutil.WriteFile(confFile, []byte(configContent), 0644)
	if err != nil {
		return "", err
	}

	return confFile, nil
}

func getPHPPortSuffix(ver string) string {
	switch ver {
	case "8.2":
		return "82"
	case "8.1":
		return "81"
	case "8.0":
		return "80"
	case "7.4":
		return "74"
	default:
		return "82"
	}
}

// RemoveNginxConfig purges the vhost record file upon domain deletion
func RemoveNginxConfig(domainName string, workspaceDir string) error {
	nginxVHostsDir := filepath.Join(workspaceDir, "nginx_vhosts")
	confFile := filepath.Join(nginxVHostsDir, fmt.Sprintf("%s.conf", domainName))
	
	// Delete only if it exists
	if _, err := os.Stat(confFile); !os.IsNotExist(err) {
		return os.Remove(confFile)
	}
	return nil
}
