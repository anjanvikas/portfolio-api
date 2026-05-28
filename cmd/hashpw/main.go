// hashpw reads a plaintext password from stdin and prints a bcrypt hash
// suitable for the ADMIN_PASSWORD env var. Usage:
//
//	echo -n 'my passphrase' | go run ./cmd/hashpw
//	# or interactively:
//	make hashpw
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	r := bufio.NewReader(os.Stdin)
	raw, err := io.ReadAll(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read stdin:", err)
		os.Exit(1)
	}
	pw := strings.TrimRight(string(raw), "\r\n")
	if pw == "" {
		fmt.Fprintln(os.Stderr, "empty password")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bcrypt:", err)
		os.Exit(1)
	}
	fmt.Println(string(hash))
}
