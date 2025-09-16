package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

// GetFileExtension returns the file extension without the dot
func GetFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if len(ext) > 0 {
		return strings.ToLower(ext[1:])
	}
	return ""
}

// IsImageFile checks if a file has an image extension
func IsImageFile(filename string) bool {
	ext := GetFileExtension(filename)
	imageExts := []string{"jpg", "jpeg", "png", "gif", "bmp", "tiff", "webp"}
	
	for _, imgExt := range imageExts {
		if ext == imgExt {
			return true
		}
	}
	return false
}

// GenerateOutputFilename generates an output filename based on input and parameters
func GenerateOutputFilename(inputFile, outputDir, prefix, suffix, format string) string {
	baseName := filepath.Base(inputFile)
	nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	
	if format == "" {
		format = GetFileExtension(inputFile)
		if format == "" {
			format = "jpg"
		}
	}
	
	outputName := fmt.Sprintf("%s%s%s.%s", prefix, nameWithoutExt, suffix, format)
	return filepath.Join(outputDir, outputName)
}

// ListImageFiles recursively lists all image files in a directory
func ListImageFiles(dir string) ([]string, error) {
	var files []string
	
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && IsImageFile(path) {
			files = append(files, path)
		}
		
		return nil
	})
	
	return files, err
}

// FileExists checks if a file exists and is not a directory
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// DirExists checks if a directory exists
func DirExists(dirname string) bool {
	info, err := os.Stat(dirname)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// SanitizeFilename removes or replaces invalid characters in filenames
func SanitizeFilename(filename string) string {
	// Replace invalid characters with underscores
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := filename
	
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	
	// Remove leading/trailing spaces and dots
	result = strings.Trim(result, " .")
	
	return result
}

// FormatFileSize formats file size in human-readable format
func FormatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}