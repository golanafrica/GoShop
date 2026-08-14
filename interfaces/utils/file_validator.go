package utils

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

// ============================================================
// FILE VALIDATOR - Validation de fichiers uploadés
// ============================================================

// FileType représente un type de fichier supporté
type FileType string

const (
	FileTypeJPEG FileType = "jpeg"
	FileTypePNG  FileType = "png"
	FileTypePDF  FileType = "pdf"
)

// MagicNumbers contient les signatures magiques des fichiers supportés
var MagicNumbers = map[FileType][]byte{
	FileTypeJPEG: {0xFF, 0xD8, 0xFF},
	FileTypePNG:  {0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
	FileTypePDF:  {0x25, 0x50, 0x44, 0x46}, // %PDF
}

// AllowedExtensions contient les extensions autorisées par type
var AllowedExtensions = map[FileType][]string{
	FileTypeJPEG: {".jpg", ".jpeg"},
	FileTypePNG:  {".png"},
	FileTypePDF:  {".pdf"},
}

// AllowedMimeTypes contient les MIME types autorisés
var AllowedMimeTypes = map[FileType]string{
	FileTypeJPEG: "image/jpeg",
	FileTypePNG:  "image/png",
	FileTypePDF:  "application/pdf",
}

// MaxFileSizes définit les tailles maximales par type (en octets)
var MaxFileSizes = map[FileType]int64{
	FileTypeJPEG: 5 * 1024 * 1024,  // 5 MB
	FileTypePNG:  5 * 1024 * 1024,  // 5 MB
	FileTypePDF:  10 * 1024 * 1024, // 10 MB
}

// ValidationResult contient le résultat de la validation
type ValidationResult struct {
	Valid        bool
	FileType     FileType
	MimeType     string
	Size         int64
	ErrorMessage string
}

// ValidateFileContent valide le contenu réel d'un fichier via magic number
func ValidateFileContent(reader io.ReadSeeker, maxSize int64, allowedTypes []FileType) (*ValidationResult, error) {
	// 1. Vérifier la taille
	size, err := reader.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get file size: %w", err)
	}

	// Rembobiner
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to rewind file: %w", err)
	}

	// 2. Vérifier taille max globale
	if size > maxSize {
		return &ValidationResult{
			Valid:        false,
			Size:         size,
			ErrorMessage: fmt.Sprintf("file size %d exceeds maximum %d bytes", size, maxSize),
		}, nil
	}

	// 3. Lire les 8 premiers octets pour magic number
	header := make([]byte, 8)
	n, err := reader.Read(header)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}
	header = header[:n]

	// Rembobiner
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to rewind file: %w", err)
	}

	// 4. Détecter le type via magic number
	var detectedType FileType
	for fileType, magic := range MagicNumbers {
		if len(header) >= len(magic) && bytes.Equal(header[:len(magic)], magic) {
			detectedType = fileType
			break
		}
	}

	if detectedType == "" {
		return &ValidationResult{
			Valid:        false,
			Size:         size,
			ErrorMessage: "unable to detect file type from magic number",
		}, nil
	}

	// 5. Vérifier que le type détecté est autorisé
	allowed := false
	for _, t := range allowedTypes {
		if t == detectedType {
			allowed = true
			break
		}
	}

	if !allowed {
		return &ValidationResult{
			Valid:        false,
			FileType:     detectedType,
			Size:         size,
			ErrorMessage: fmt.Sprintf("file type %s not allowed", detectedType),
		}, nil
	}

	// 6. Vérifier taille max spécifique au type
	if size > MaxFileSizes[detectedType] {
		return &ValidationResult{
			Valid:        false,
			FileType:     detectedType,
			Size:         size,
			ErrorMessage: fmt.Sprintf("file size %d exceeds maximum %d bytes for type %s", size, MaxFileSizes[detectedType], detectedType),
		}, nil
	}

	return &ValidationResult{
		Valid:    true,
		FileType: detectedType,
		MimeType: AllowedMimeTypes[detectedType],
		Size:     size,
	}, nil
}

// ValidateFileExtension valide l'extension du fichier
func ValidateFileExtension(filename string, allowedTypes []FileType) error {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return fmt.Errorf("file has no extension")
	}

	for _, fileType := range allowedTypes {
		for _, allowedExt := range AllowedExtensions[fileType] {
			if ext == allowedExt {
				return nil
			}
		}
	}

	return fmt.Errorf("file extension %s not allowed", ext)
}

// ValidateFileMetadata valide la cohérence entre extension et contenu
func ValidateFileMetadata(filename, declaredMimeType string, detectedType FileType) error {
	// Vérifier extension
	ext := strings.ToLower(filepath.Ext(filename))
	expectedExt := AllowedExtensions[detectedType]

	extValid := false
	for _, e := range expectedExt {
		if ext == e {
			extValid = true
			break
		}
	}

	if !extValid {
		return fmt.Errorf("file extension %s does not match detected type %s", ext, detectedType)
	}

	// Vérifier MIME type déclaré vs détecté
	expectedMimeType := AllowedMimeTypes[detectedType]
	if declaredMimeType != "" && declaredMimeType != expectedMimeType {
		return fmt.Errorf("declared mime type %s does not match detected type %s (expected %s)",
			declaredMimeType, detectedType, expectedMimeType)
	}

	return nil
}

// DetectContentType utilise http.DetectContentType comme fallback
func DetectContentType(data []byte) string {
	return http.DetectContentType(data)
}
