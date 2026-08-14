package utils

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// TESTS : ValidateFileContent (Magic Number Validation)
// ============================================================

func TestValidateFileContent_ValidJPEG(t *testing.T) {
	// Magic number JPEG : FF D8 FF
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	reader := bytes.NewReader(jpegData)

	result, err := ValidateFileContent(reader, 5*1024*1024, []FileType{FileTypeJPEG, FileTypePNG, FileTypePDF})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Equal(t, FileTypeJPEG, result.FileType)
	assert.Equal(t, "image/jpeg", result.MimeType)
}

func TestValidateFileContent_ValidPNG(t *testing.T) {
	// Magic number PNG : 89 50 4E 47 0D 0A 1A 0A
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	reader := bytes.NewReader(pngData)

	result, err := ValidateFileContent(reader, 5*1024*1024, []FileType{FileTypeJPEG, FileTypePNG, FileTypePDF})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Equal(t, FileTypePNG, result.FileType)
	assert.Equal(t, "image/png", result.MimeType)
}

func TestValidateFileContent_ValidPDF(t *testing.T) {
	// Magic number PDF : 25 50 44 46 (%PDF)
	pdfData := []byte{0x25, 0x50, 0x44, 0x46, 0x2D, 0x31, 0x2E, 0x34}
	reader := bytes.NewReader(pdfData)

	result, err := ValidateFileContent(reader, 10*1024*1024, []FileType{FileTypeJPEG, FileTypePNG, FileTypePDF})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Equal(t, FileTypePDF, result.FileType)
	assert.Equal(t, "application/pdf", result.MimeType)
}

func TestValidateFileContent_InvalidMagicNumber(t *testing.T) {
	// Données invalides (pas de magic number connu)
	invalidData := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	reader := bytes.NewReader(invalidData)

	result, err := ValidateFileContent(reader, 5*1024*1024, []FileType{FileTypeJPEG, FileTypePNG, FileTypePDF})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Valid)
	assert.Contains(t, result.ErrorMessage, "unable to detect file type")
}

func TestValidateFileContent_FileTooLarge(t *testing.T) {
	// Fichier de 6 MB (dépasse la limite de 5 MB)
	largeData := make([]byte, 6*1024*1024)
	// Ajouter magic number JPEG au début
	largeData[0] = 0xFF
	largeData[1] = 0xD8
	largeData[2] = 0xFF
	reader := bytes.NewReader(largeData)

	result, err := ValidateFileContent(reader, 5*1024*1024, []FileType{FileTypeJPEG})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Valid)
	assert.Contains(t, result.ErrorMessage, "exceeds maximum")
}

func TestValidateFileContent_NotAllowedType(t *testing.T) {
	// JPEG valide mais seul PNG est autorisé
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	reader := bytes.NewReader(jpegData)

	result, err := ValidateFileContent(reader, 5*1024*1024, []FileType{FileTypePNG}) // Seul PNG autorisé

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Valid)
	assert.Contains(t, result.ErrorMessage, "not allowed")
}

// ============================================================
// TESTS : ValidateFileExtension
// ============================================================

func TestValidateFileExtension_ValidJPEG(t *testing.T) {
	err := ValidateFileExtension("photo.jpg", []FileType{FileTypeJPEG, FileTypePNG})
	assert.NoError(t, err)

	err = ValidateFileExtension("photo.jpeg", []FileType{FileTypeJPEG})
	assert.NoError(t, err)
}

func TestValidateFileExtension_ValidPNG(t *testing.T) {
	err := ValidateFileExtension("image.png", []FileType{FileTypePNG})
	assert.NoError(t, err)
}

func TestValidateFileExtension_ValidPDF(t *testing.T) {
	err := ValidateFileExtension("document.pdf", []FileType{FileTypePDF})
	assert.NoError(t, err)
}

func TestValidateFileExtension_NoExtension(t *testing.T) {
	err := ValidateFileExtension("filename", []FileType{FileTypeJPEG})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no extension")
}

func TestValidateFileExtension_InvalidExtension(t *testing.T) {
	err := ValidateFileExtension("malware.exe", []FileType{FileTypeJPEG, FileTypePNG})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
}

func TestValidateFileExtension_CaseInsensitive(t *testing.T) {
	// Doit accepter .JPG, .PNG, .PDF (majuscules)
	err := ValidateFileExtension("PHOTO.JPG", []FileType{FileTypeJPEG})
	assert.NoError(t, err)

	err = ValidateFileExtension("IMAGE.PNG", []FileType{FileTypePNG})
	assert.NoError(t, err)
}

// ============================================================
// TESTS : ValidateFileMetadata (Cohérence extension/contenu)
// ============================================================

func TestValidateFileMetadata_ConsistentJPEG(t *testing.T) {
	err := ValidateFileMetadata("photo.jpg", "image/jpeg", FileTypeJPEG)
	assert.NoError(t, err)
}

func TestValidateFileMetadata_ConsistentPNG(t *testing.T) {
	err := ValidateFileMetadata("image.png", "image/png", FileTypePNG)
	assert.NoError(t, err)
}

func TestValidateFileMetadata_ConsistentPDF(t *testing.T) {
	err := ValidateFileMetadata("document.pdf", "application/pdf", FileTypePDF)
	assert.NoError(t, err)
}

func TestValidateFileMetadata_ExtensionMismatch(t *testing.T) {
	// Extension .png mais contenu détecté comme JPEG
	err := ValidateFileMetadata("fake.png", "image/jpeg", FileTypeJPEG)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestValidateFileMetadata_MimeTypeMismatch(t *testing.T) {
	// MIME déclaré image/png mais contenu détecté comme JPEG
	err := ValidateFileMetadata("photo.jpg", "image/png", FileTypeJPEG)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestValidateFileMetadata_EmptyMimeType(t *testing.T) {
	// MIME vide = pas de validation MIME (seulement extension)
	err := ValidateFileMetadata("photo.jpg", "", FileTypeJPEG)
	assert.NoError(t, err)
}
