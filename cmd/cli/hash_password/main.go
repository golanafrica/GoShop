package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run hash_password.go <password>")
		os.Exit(1)
	}

	password := os.Args[1]

	// Utiliser le coût par défaut (12 en production)
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n========================================\n")
	fmt.Printf("Password original : %s\n", password)
	fmt.Printf("Hash bcrypt       : %s\n", string(hashed))
	fmt.Printf("Longueur          : %d caractères\n", len(string(hashed)))
	fmt.Printf("========================================\n\n")
	fmt.Printf("📋 Copiez ce hash dans votre migration SQL :\n")
	fmt.Printf("   UPDATE users SET password = '%s' WHERE email = 'admin@goshop.com';\n\n", string(hashed))
}
