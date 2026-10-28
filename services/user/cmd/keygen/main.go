package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	out := flag.String("out", ".secrets/jwt_ed25519.key", "output path for the base64-encoded seed")
	force := flag.Bool("force", false, "overwrite an existing key")
	flag.Parse()

	if err := run(*out, *force); err != nil {
		fmt.Fprintln(os.Stderr, "keygen:", err)
		os.Exit(1)
	}
}

func run(out string, force bool) error {
	if _, err := os.Stat(out); err == nil && !force {
		return fmt.Errorf("%s already exists; use -force to overwrite", out)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil {
		return err
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	seed := privateKey.Seed()

	encoded := base64.StdEncoding.EncodeToString(seed) + "\n"
	if err := os.WriteFile(out, []byte(encoded), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(out, 0o600); err != nil {
		return err
	}

	fmt.Printf("wrote Ed25519 seed to %s\n", out)
	fmt.Println("load it in the API environment with:")
	fmt.Printf("  export JWT_PRIVATE_KEY=\"$(cat %s)\"\n", out)
	return nil
}
