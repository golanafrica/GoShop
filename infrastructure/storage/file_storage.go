package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// FileStorage gère le stockage local des fichiers
type FileStorage struct {
	basePath string
}

// NewFileStorage crée une nouvelle instance
func NewFileStorage(basePath string) (*FileStorage, error) {
	// Créer le dossier s'il n'existe pas
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &FileStorage{basePath: basePath}, nil
}

// Save sauvegarde un fichier et retourne son chemin relatif
func (fs *FileStorage) Save(reader io.Reader, originalFilename, extension string) (string, error) {
	// Générer un nom unique
	timestamp := time.Now().UnixNano()
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	randomHex := hex.EncodeToString(randomBytes)

	filename := fmt.Sprintf("%d_%s%s", timestamp, randomHex, extension)

	// Créer les sous-dossiers par date (YYYY/MM/DD)
	now := time.Now()
	datePath := filepath.Join(
		fmt.Sprintf("%04d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
		fmt.Sprintf("%02d", now.Day()),
	)

	fullDir := filepath.Join(fs.basePath, datePath)
	if err := os.MkdirAll(fullDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create date directory: %w", err)
	}

	fullPath := filepath.Join(fullDir, filename)

	// Créer le fichier
	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copier le contenu
	if _, err := io.Copy(file, reader); err != nil {
		// Nettoyer en cas d'erreur
		os.Remove(fullPath)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Retourner le chemin relatif
	relativePath := filepath.Join(datePath, filename)
	return relativePath, nil
}

// Delete supprime un fichier
func (fs *FileStorage) Delete(relativePath string) error {
	fullPath := filepath.Join(fs.basePath, relativePath)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// GetFullPath retourne le chemin absolu d'un fichier
func (fs *FileStorage) GetFullPath(relativePath string) string {
	return filepath.Join(fs.basePath, relativePath)
}

// Exists vérifie si un fichier existe
func (fs *FileStorage) Exists(relativePath string) bool {
	fullPath := filepath.Join(fs.basePath, relativePath)
	_, err := os.Stat(fullPath)
	return err == nil
}
