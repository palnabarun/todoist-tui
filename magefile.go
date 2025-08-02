//go:build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	appName = "todoist-tui"
	version = "1.0.0"
)

// Default target to run when none is specified
var Default = Build

// Build builds the binary for the current platform
func Build() error {
	fmt.Println("Building", appName, "for", runtime.GOOS+"/"+runtime.GOARCH)

	ldflags := fmt.Sprintf("-X main.version=%s", version)
	env := map[string]string{
		"CGO_ENABLED": "0",
	}

	return sh.RunWith(env, "go", "build", "-ldflags", ldflags, "-o", appName)
}

// Clean removes build artifacts
func Clean() error {
	fmt.Println("Cleaning build artifacts...")

	artifacts := []string{
		appName,
		appName + ".exe",
		"dist/",
	}

	for _, artifact := range artifacts {
		if err := sh.Rm(artifact); err != nil {
			// Ignore errors for files that don't exist
			continue
		}
	}

	return nil
}

// Test runs the test suite
func Test() error {
	fmt.Println("Running tests...")
	return sh.Run("go", "test", "./...")
}

// Lint runs go fmt and go vet
func Lint() error {
	fmt.Println("Running go fmt...")
	if err := sh.Run("go", "fmt", "./..."); err != nil {
		return err
	}

	fmt.Println("Running go vet...")
	return sh.Run("go", "vet", "./...")
}

// LintCI runs golangci-lint in a container
func LintCI() error {
	fmt.Println("Running golangci-lint in container...")
	
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	
	// Use the official golangci-lint Docker image
	return sh.Run("docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/app", pwd),
		"-w", "/app",
		"golangci/golangci-lint:v1.61.0",
		"golangci-lint", "run", "-v")
}

// LintCIFix runs golangci-lint in a container with --fix
func LintCIFix() error {
	fmt.Println("Running golangci-lint in container with --fix...")
	
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	
	// Use the official golangci-lint Docker image
	return sh.Run("docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/app", pwd),
		"-w", "/app",
		"golangci/golangci-lint:v1.61.0",
		"golangci-lint", "run", "--fix", "-v")
}

// BuildAll builds binaries for all supported platforms
func BuildAll() error {
	fmt.Println("Building for all platforms...")

	mg.Deps(Clean)

	if err := os.MkdirAll("dist", 0755); err != nil {
		return err
	}

	platforms := []struct {
		goos, goarch string
	}{
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"windows", "amd64"},
		{"windows", "arm64"},
	}

	ldflags := fmt.Sprintf("-X main.version=%s", version)

	for _, platform := range platforms {
		binaryName := appName
		if platform.goos == "windows" {
			binaryName += ".exe"
		}

		outputPath := filepath.Join("dist", fmt.Sprintf("%s-%s-%s", appName, platform.goos, platform.goarch))
		if platform.goos == "windows" {
			outputPath += ".exe"
		}

		fmt.Printf("Building %s/%s -> %s\n", platform.goos, platform.goarch, outputPath)

		env := map[string]string{
			"GOOS":        platform.goos,
			"GOARCH":      platform.goarch,
			"CGO_ENABLED": "0",
		}

		if err := sh.RunWith(env, "go", "build", "-ldflags", ldflags, "-o", outputPath); err != nil {
			return fmt.Errorf("failed to build %s/%s: %w", platform.goos, platform.goarch, err)
		}
	}

	fmt.Println("All builds completed successfully!")
	return nil
}

// Install builds and installs the binary to $GOPATH/bin or $GOBIN
func Install() error {
	fmt.Println("Installing", appName)

	ldflags := fmt.Sprintf("-X main.version=%s", version)
	env := map[string]string{
		"CGO_ENABLED": "0",
	}

	return sh.RunWith(env, "go", "install", "-ldflags", ldflags)
}

// Dev runs the application in development mode
func Dev() error {
	fmt.Println("Running in development mode...")
	return sh.Run("go", "run", ".")
}

// Deps downloads and tidy dependencies
func Deps() error {
	fmt.Println("Downloading dependencies...")
	if err := sh.Run("go", "mod", "download"); err != nil {
		return err
	}

	fmt.Println("Tidying dependencies...")
	return sh.Run("go", "mod", "tidy")
}

// Check runs all quality checks (lint, test)
func Check() error {
	mg.Deps(Lint, Test)
	return nil
}

// CheckCI runs all quality checks with golangci-lint in container
func CheckCI() error {
	mg.Deps(LintCI, Test)
	return nil
}

// Release prepares a release build
func Release() error {
	fmt.Println("Preparing release...")
	mg.Deps(Check, BuildAll)
	return nil
}
