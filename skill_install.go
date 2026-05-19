package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const embeddedSkillName = "gh-pr-review"
const defaultSkillInstallDirValue = "~/.agent/skills"

//go:embed skills/gh-pr-review/**
var embeddedSkillFiles embed.FS

func runInstallSkill(args []string) error {
	fs := flag.NewFlagSet("install-skill", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { printInstallSkillUsage(fs.Output()) }

	var dir string
	fs.StringVar(&dir, "dir", defaultSkillInstallDirValue, "Install root for skills")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	installedPath, err := installEmbeddedSkill(dir)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "installed skill %q to %s\n", embeddedSkillName, installedPath)
	return nil
}

func printInstallSkillUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: gh-pr-review install-skill [--dir <path>]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Installs the embedded gh-pr-review Hermes skill into the target skills directory.")
}

func defaultSkillInstallDir() (string, error) {
	return expandHomeDir(defaultSkillInstallDirValue)
}

func installEmbeddedSkill(dir string) (string, error) {
	dir, err := expandHomeDir(dir)
	if err != nil {
		return "", err
	}

	sourceRoot := filepath.ToSlash(filepath.Join("skills", embeddedSkillName))
	destinationRoot := filepath.Join(dir, embeddedSkillName)
	if err := copyEmbeddedTree(sourceRoot, destinationRoot); err != nil {
		return "", err
	}
	return destinationRoot, nil
}

func expandHomeDir(path string) (string, error) {
	switch {
	case path == "":
		return "", errors.New("install directory is required")
	case path == "~":
		return os.UserHomeDir()
	case strings.HasPrefix(path, "~/"):
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(homeDir, path[2:]), nil
	default:
		return path, nil
	}
}

func copyEmbeddedTree(sourceRoot, destinationRoot string) error {
	return fs.WalkDir(embeddedSkillFiles, sourceRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		targetPath := destinationRoot
		if relativePath != "." {
			targetPath = filepath.Join(destinationRoot, filepath.FromSlash(relativePath))
		}

		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		data, err := embeddedSkillFiles.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0o644)
	})
}
