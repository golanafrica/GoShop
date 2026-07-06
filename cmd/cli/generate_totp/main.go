package main

import (
	"fmt"
	"os"
	"time"

	"github.com/pquerna/otp/totp"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run cmd/cli/generate_totp/main.go <SECRET>")
		fmt.Println("Example: go run cmd/cli/generate_totp/main.go JBSWY3DPEHPK3PXP")
		os.Exit(1)
	}

	secret := os.Args[1]

	// 🆕 Correction : utiliser time.Now() au lieu de nil
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(code)
}
