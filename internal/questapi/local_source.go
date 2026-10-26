package questapi

import (
	"os"
	"path/filepath"
)

// Starfall adds quest API methods and events that upstream EQEmu/Server does
// not have, and its repository is private, so the API is parsed from the
// server source on this machine: the same source "build from source" uses.

// directories the parsers read; everything they look for lives in these
var localSourceDirs = []string{"zone", "common"}

// localServerSource returns the server source root and its files, keyed by
// path relative to the root (eg zone/perl_client.cpp), the same keys the
// GitHub source download produces. It returns an empty root when no local
// source is found.
func (c *ParseService) localServerSource() (string, map[string]string) {
	root := c.updater.GetSourceRoot()
	if root == "" {
		return "", nil
	}

	files := readServerSource(root)
	if files == nil {
		return "", nil
	}

	return root, files
}

// readServerSource reads the parsed directories of a server checkout, or
// returns nil when root does not look like one
func readServerSource(root string) map[string]string {
	if !isServerSource(root) {
		return nil
	}

	files := map[string]string{}
	for _, dir := range localSourceDirs {
		_ = filepath.Walk(
			filepath.Join(root, dir),
			func(path string, fi os.FileInfo, err error) error {
				if err != nil || !fi.Mode().IsRegular() {
					return nil
				}

				ext := filepath.Ext(path)
				if ext != ".cpp" && ext != ".h" {
					return nil
				}

				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}

				rel, err := filepath.Rel(root, path)
				if err != nil {
					return nil
				}

				files[filepath.ToSlash(rel)] = string(data)

				return nil
			},
		)
	}

	return files
}

// isServerSource reports whether root looks like a server source checkout
func isServerSource(root string) bool {
	_, err := os.Stat(filepath.Join(root, "zone", "embparser.cpp"))
	return err == nil
}
