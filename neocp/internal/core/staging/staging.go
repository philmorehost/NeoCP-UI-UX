package staging

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"neocp/internal/core"
)

// CopyFile copies a single file from src to dst
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// CopyDir recursively copies a directory tree
func CopyDir(src string, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	directory, err := os.Open(src)
	if err != nil {
		return err
	}
	defer directory.Close()

	objects, err := directory.Readdir(-1)
	if err != nil {
		return err
	}

	for _, obj := range objects {
		srcFilePath := filepath.Join(src, obj.Name())
		dstFilePath := filepath.Join(dst, obj.Name())

		if obj.IsDir() {
			if err = CopyDir(srcFilePath, dstFilePath); err != nil {
				return err
			}
		} else {
			if err = CopyFile(srcFilePath, dstFilePath); err != nil {
				return err
			}
		}
	}
	return nil
}

// RewritePayload performs serialization-aware replacement of domains and databases
func RewritePayload(content string, oldDomain, newDomain, oldDB, newDB string) string {
	// 1. Serialize-aware PHP string replacement: s:<len>:"...<old>..."
	// Matches standard serialized PHP string variables non-greedily
	re := regexp.MustCompile(`s:(\d+):\"(.*?)\"`)

	rewritten := re.ReplaceAllStringFunc(content, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		strVal := submatches[2]

		modified := false
		newValStr := strVal
		if oldDomain != "" && strings.Contains(newValStr, oldDomain) {
			newValStr = strings.ReplaceAll(newValStr, oldDomain, newDomain)
			modified = true
		}
		if oldDB != "" && strings.Contains(newValStr, oldDB) {
			newValStr = strings.ReplaceAll(newValStr, oldDB, newDB)
			modified = true
		}

		if modified {
			return fmt.Sprintf(`s:%d:"%s"`, len(newValStr), newValStr)
		}
		return match
	})

	// 2. Global direct search and replace for other normal text occurrences
	if oldDomain != "" {
		rewritten = strings.ReplaceAll(rewritten, oldDomain, newDomain)
	}
	if oldDB != "" {
		rewritten = strings.ReplaceAll(rewritten, oldDB, newDB)
	}

	return rewritten
}

// ProcessAndRewriteFiles walks through a directory and applies RewritePayload to configuration/text files
func ProcessAndRewriteFiles(dir, oldDomain, newDomain, oldDB, newDB string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Rewrite only relevant text config/code files to save time and prevent binary corruption
		ext := strings.ToLower(filepath.Ext(path))
		name := strings.ToLower(info.Name())
		if ext == ".php" || ext == ".env" || ext == ".htaccess" || ext == ".json" || ext == ".xml" || ext == ".conf" || ext == ".ini" || ext == ".sql" || strings.Contains(name, "config") {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return nil // skip unreadable files
			}

			rewritten := RewritePayload(string(data), oldDomain, newDomain, oldDB, newDB)
			_ = ioutil.WriteFile(path, []byte(rewritten), info.Mode())
		}
		return nil
	})
}

// CloneToStaging creates a full replica of production to staging
func CloneToStaging(owner, productionDomain, stagingSubdomain, sandboxDir string) error {
	db := core.GetDB()

	// 1. Verify production domain exists
	prodDomains := db.GetDomains(owner, false)
	var prodDom *core.Domain
	for _, d := range prodDomains {
		if d.DomainName == productionDomain {
			prodDom = &d
			break
		}
	}
	if prodDom == nil {
		return errors.New("production domain not found")
	}

	// 2. Local File Copy
	prodPath := filepath.Join(sandboxDir, owner, "public_html", productionDomain)
	stagePath := filepath.Join(sandboxDir, owner, "public_html", stagingSubdomain)

	// Create prod path if missing for safety
	_ = os.MkdirAll(prodPath, 0755)

	if err := CopyDir(prodPath, stagePath); err != nil {
		return fmt.Errorf("failed to copy files to staging: %w", err)
	}

	// 3. Database Cloning
	ownerDBs := db.GetDatabases(owner, false)
	var oldDBName, newDBName string

	// Look for the database related to the owner
	for _, database := range ownerDBs {
		// Only copy database if it belongs to the domain (e.g. contains prefix or starts with owner)
		if strings.HasPrefix(database.Name, owner+"_") && !strings.Contains(database.Name, "_staging_") {
			oldDBName = database.Name
			// e.g. patel_wpblog -> patel_staging_wpblog
			newDBName = strings.Replace(oldDBName, owner+"_", owner+"_staging_", 1)

			// Clone Database
			clonedDB := core.Database{
				Name:      newDBName,
				Owner:     owner,
				DBUser:    database.DBUser + "_stage",
				Password:  database.Password,
				RemoteIPs: database.RemoteIPs,
				CreatedAt: time.Now(),
			}
			_ = db.CreateDatabase(clonedDB)
			break
		}
	}

	// 4. Update configuration files in staging web root
	if err := ProcessAndRewriteFiles(stagePath, productionDomain, stagingSubdomain, oldDBName, newDBName); err != nil {
		return fmt.Errorf("failed to rewrite configuration files: %w", err)
	}

	// 5. Register Staging Domain in Panel DB
	stageDom := core.Domain{
		DomainName:    stagingSubdomain,
		Owner:         owner,
		PHPVersion:    prodDom.PHPVersion,
		SSLActive:     true,
		SSLIssuer:     "NeoCP Staging Authority",
		SSLExpires:    time.Now().AddDate(0, 3, 0),
		WebDAVEnabled: prodDom.WebDAVEnabled,
		GzipEnabled:   prodDom.GzipEnabled,
		BrotliEnabled: prodDom.BrotliEnabled,
		CreatedAt:     time.Now(),
	}
	_ = db.CreateDomain(stageDom)

	return nil
}

// PushStagingToProduction pushes changes from staging back to production with safety backups
func PushStagingToProduction(owner, stagingSubdomain, syncMode, sandboxDir string) error {
	db := core.GetDB()

	// 1. Verify staging domain exists
	domains := db.GetDomains(owner, false)
	var stageDom *core.Domain
	for _, d := range domains {
		if d.DomainName == stagingSubdomain {
			stageDom = &d
			break
		}
	}
	if stageDom == nil {
		return errors.New("staging domain not found")
	}

	// Resolve production domain. Staging domain is usually sub.domain.com, prod domain is domain.com
	// In NeoCP staging convention: e.g. blog-stage.digitalneo.net -> blog.digitalneo.net
	prodDomainName := strings.Replace(stagingSubdomain, "-stage", "", 1)
	if prodDomainName == stagingSubdomain {
		prodDomainName = strings.Replace(stagingSubdomain, "staging.", "", 1)
	}

	var prodDom *core.Domain
	for _, d := range domains {
		if d.DomainName == prodDomainName {
			prodDom = &d
			break
		}
	}
	if prodDom == nil {
		return fmt.Errorf("corresponding production domain (%s) not found", prodDomainName)
	}

	prodPath := filepath.Join(sandboxDir, owner, "public_html", prodDomainName)
	stagePath := filepath.Join(sandboxDir, owner, "public_html", stagingSubdomain)

	// Resolve databases
	ownerDBs := db.GetDatabases(owner, false)
	var prodDBName, stageDBName string
	for _, database := range ownerDBs {
		if strings.HasPrefix(database.Name, owner+"_staging_") {
			stageDBName = database.Name
			prodDBName = strings.Replace(stageDBName, owner+"_staging_", owner+"_", 1)
			break
		}
	}

	// 2. Perform Pre-Push Backup
	backupPath := filepath.Join(sandboxDir, owner, "backups")
	_ = os.MkdirAll(backupPath, 0755)
	prePushArchive := filepath.Join(backupPath, fmt.Sprintf("prepush_%s_%d.bak", prodDomainName, time.Now().Unix()))

	// Create backup of production files
	_ = CopyDir(prodPath, prePushArchive)

	// 3. Perform Sync actions based on Mode
	if syncMode == "files" || syncMode == "both" {
		// Sync Files: Remove production files and replace with copied staging files
		_ = os.RemoveAll(prodPath)
		if err := CopyDir(stagePath, prodPath); err != nil {
			return fmt.Errorf("failed to sync staging files to production: %w", err)
		}
		// Rewrite domains back (Staging Subdomain -> Production Domain)
		_ = ProcessAndRewriteFiles(prodPath, stagingSubdomain, prodDomainName, stageDBName, prodDBName)
	}

	if syncMode == "db" || syncMode == "both" {
		// Sync Database: In simulated environment, database values are mapped in json or config files.
		// Since config files are already rewritten in files sync, if user requested only "db", we rewrite
		// DB credentials/values in the production folder from staging DB to production DB.
		if syncMode == "db" {
			// Copy staging config files back to production to simulate db credentials push
			_ = ProcessAndRewriteFiles(prodPath, stagingSubdomain, prodDomainName, stageDBName, prodDBName)
		}
	}

	return nil
}

// DeleteStaging cleans up the staging environment folder and database definitions
func DeleteStaging(owner, stagingSubdomain, sandboxDir string) error {
	db := core.GetDB()

	// Delete from domain database
	_ = db.DeleteDomain(stagingSubdomain)

	// Remove staging filesystem root
	stagePath := filepath.Join(sandboxDir, owner, "public_html", stagingSubdomain)
	_ = os.RemoveAll(stagePath)

	// Remove staging database
	ownerDBs := db.GetDatabases(owner, false)
	for _, database := range ownerDBs {
		if database.Owner == owner && strings.HasPrefix(database.Name, owner+"_staging_") {
			_ = db.DeleteDatabase(database.Name)
			break
		}
	}

	return nil
}
