package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
)

var (
	ErrEncryptionKeyNotSet = errors.New("ENCRYPTION_KEY environment variable not set")
	ErrInvalidCipherText   = errors.New("invalid ciphertext")
	ErrEmptyValue          = errors.New("cannot encrypt empty value")
)

// getEncryptionKey récupère la clé de chiffrement depuis l'environnement
// La clé doit faire 32 bytes (256 bits) pour AES-256
func getEncryptionKey() ([]byte, error) {
	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		return nil, ErrEncryptionKeyNotSet
	}

	// Si la clé fait exactement 32 caractères, on l'utilise directement
	if len(key) == 32 {
		return []byte(key), nil
	}

	// Sinon, on la décode depuis base64
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, errors.New("ENCRYPTION_KEY must be 32 chars or base64-encoded 32 bytes")
	}

	if len(decoded) != 32 {
		return nil, errors.New("ENCRYPTION_KEY must be exactly 32 bytes")
	}

	return decoded, nil
}

// Encrypt chiffre une valeur avec AES-256-GCM
// Retourne le ciphertext encodé en base64
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", ErrEmptyValue
	}

	key, err := getEncryptionKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Générer un nonce unique pour chaque chiffrement
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Chiffrer : nonce + ciphertext
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encoder en base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt déchiffre une valeur chiffrée avec AES-256-GCM
func Decrypt(encrypted string) (string, error) {
	if encrypted == "" {
		return "", ErrEmptyValue
	}

	key, err := getEncryptionKey()
	if err != nil {
		return "", err
	}

	// Décoder depuis base64
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", ErrInvalidCipherText
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", ErrInvalidCipherText
	}

	// Séparer nonce et ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Déchiffrer
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrInvalidCipherText
	}

	return string(plaintext), nil
}

// EncryptOrEmpty chiffre une valeur, retourne "" si vide
func EncryptOrEmpty(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	return Encrypt(plaintext)
}

// DecryptOrEmpty déchiffre une valeur, retourne "" si vide
func DecryptOrEmpty(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}
	return Decrypt(encrypted)
}
