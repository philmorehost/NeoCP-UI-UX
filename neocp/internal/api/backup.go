package api

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BackupItem represents archive metadata
type BackupItem struct {
	Filename    string `json:"filename"`
	SizeMB      float64 `json:"size_mb"`
	CreatedAt   string `json:"created_at"`
}

// CreateBackupZip bundles the specified user directory safely into a single ZIP archive
func CreateBackupZip(username string, sandboxDir string, targetZipPath string) error {
	zipFile, err := os.Create(targetZipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	userDir := filepath.Join(sandboxDir, username)

	err = filepath.Walk(userDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the backups directory itself to prevent recursive backup size loops
		if strings.Contains(filepath.ToSlash(path), "/backups") {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(userDir, path)
		if err != nil {
			return err
		}
		
		header.Name = filepath.ToSlash(relPath)

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	return err
}

// RestoreBackupZip extracts all archived elements back into the user directory, purging old files
func RestoreBackupZip(targetZipPath string, username string, sandboxDir string) error {
	userDir := filepath.Join(sandboxDir, username)

	archive, err := zip.OpenReader(targetZipPath)
	if err != nil {
		return err
	}
	defer archive.Close()

	// 1. Purge current content safely (excluding /backups directory)
	files, err := ioutil.ReadDir(userDir)
	if err == nil {
		for _, f := range files {
			if f.Name() == "backups" {
				continue
			}
			os.RemoveAll(filepath.Join(userDir, f.Name()))
		}
	}

	// 2. Extract ZIP contents
	for _, f := range archive.File {
		filePath := filepath.Join(userDir, f.Name)

		// Security: Prevent zip slip traversal exploits
		if !strings.HasPrefix(filePath, filepath.Clean(userDir)+string(os.PathSeparator)) && filePath != userDir {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		fileInArchive, err := f.Open()
		if err != nil {
			dstFile.Close()
			return err
		}

		_, err = io.Copy(dstFile, fileInArchive)
		dstFile.Close()
		fileInArchive.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// HandleBackupCreate executes backup archives triggers
func HandleBackupCreate(w http.ResponseWriter, r *http.Request, sandboxDir string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.Header.Get("NeoCP-User")
	backupsDir := filepath.Join(sandboxDir, username, "backups")
	os.MkdirAll(backupsDir, 0755)

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("backup_%s_%s.zip", username, timestamp)
	targetZipPath := filepath.Join(backupsDir, filename)

	// Create backup zip synchronously for reliable developer validation
	err := CreateBackupZip(username, sandboxDir, targetZipPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Backup failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(fmt.Sprintf(`{"success":true, "filename":"%s"}`, filename)))
}

// HandleBackupList returns list of all archived archives in customer's backups directory
func HandleBackupList(w http.ResponseWriter, r *http.Request, sandboxDir string) {
	username := r.Header.Get("NeoCP-User")
	backupsDir := filepath.Join(sandboxDir, username, "backups")
	os.MkdirAll(backupsDir, 0755)

	files, err := ioutil.ReadDir(backupsDir)
	if err != nil {
		http.Error(w, "Failed to read backups folder", http.StatusInternalServerError)
		return
	}

	var items []BackupItem
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".zip") {
			items = append(items, BackupItem{
				Filename:  f.Name(),
				SizeMB:    float64(f.Size()) / (1024 * 1024),
				CreatedAt: f.ModTime().Format("2006-01-02 15:04:05"),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// HandleBackupRestore triggers restoration mapping override
func HandleBackupRestore(w http.ResponseWriter, r *http.Request, sandboxDir string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request payload", http.StatusBadRequest)
		return
	}

	username := r.Header.Get("NeoCP-User")
	targetZipPath := filepath.Join(sandboxDir, username, "backups", req.Filename)

	// Validate file boundaries
	if _, err := os.Stat(targetZipPath); os.IsNotExist(err) {
		http.Error(w, "Selected backup file does not exist", http.StatusNotFound)
		return
	}

	err := RestoreBackupZip(targetZipPath, username, sandboxDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Restoration failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}
