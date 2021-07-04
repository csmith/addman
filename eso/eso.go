package eso

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func GuessDirectory() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	suffix := []string{"Elder Scrolls Online", "live", "AddOns"}
	protonSuffix := append([]string{"steamapps", "compatdata", "306130", "pfx", "drive_c", "users", "steamuser", "My Documents"}, suffix...)
	paths := [][]string {
		append([]string{"Documents"}, suffix...),
		append([]string{".local", "share", "Steam"}, protonSuffix...),
		append([]string{".steam", "steam"}, protonSuffix...),
	}

	for i := range paths {
		full := filepath.Join(append([]string{home}, paths[i]...)...)
		info, err := os.Stat(full)
		if err == nil && info.IsDir() {
			return full
		}
	}

	return ""
}

func InstalledAddons(addonDir string) ([]*Addon, error) {
	files, err := os.ReadDir(addonDir)
	if err != nil {
		return nil, err
	}

	var addons []*Addon
	for i := range files {
		if files[i].IsDir() {
			addons = append(addons, ReadAddon(filepath.Join(addonDir, files[i].Name())))
		}
	}

	return addons, nil
}

func ReadAddon(dir string) *Addon {
	name := filepath.Base(dir)
	meta, err := readMetadata(dir)
	if err != nil {
		return &Addon{
			Name:  name,
			Error: fmt.Errorf("unable to read metadata: %w", err),
		}
	}

	isLibrary, err := meta.isLibrary()
	if err != nil {
		return &Addon{
			Name:  name,
			Error: fmt.Errorf("unable to read IsLibrary field: %w", err),
		}
	}

	addonVersion, err := meta.addonVersion()
	if err != nil {
		return &Addon{
			Name:  name,
			Error: fmt.Errorf("unable to read AddonVersion field: %w", err),
		}
	}

	deps, err := meta.dependencies()
	if err != nil {
		return &Addon{
			Name:  name,
			Error: fmt.Errorf("unable to read DependsOn field: %w", err),
		}
	}

	return &Addon{
		Name:           name,
		Title:          meta["title"],
		Library:        isLibrary,
		DisplayVersion: meta["version"],
		Version:        addonVersion,
		DependsOn:      deps,
	}
}

func readMetadata(dir string) (metadata, error) {
	name := filepath.Base(dir)
	f, err := os.Open(filepath.Join(dir, fmt.Sprintf("%s.txt", name)))
	if err != nil {
		return nil, fmt.Errorf("unable to read metadata: %w", err)
	}
	defer f.Close()

	meta := make(metadata)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "## ") {
			if parts := strings.SplitN(strings.TrimPrefix(line, "## "), ":", 2); len(parts) == 2 {
				meta[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
			}
		}
	}

	return meta, scanner.Err()
}

type Addon struct {
	Name           string
	Title          string
	Library        bool
	DisplayVersion string
	Version        int
	DependsOn      []Dependency
	Error          error
}

type Dependency struct {
	Name           string
	MinimumVersion int
}

type metadata map[string]string

func (m metadata) isLibrary() (bool, error) {
	if val, ok := m["islibrary"]; ok {
		return strconv.ParseBool(val)
	}
	return false, nil
}

func (m metadata) addonVersion() (int, error) {
	if val, ok := m["addonversion"]; ok {
		return strconv.Atoi(val)
	}
	return 0, nil
}

func (m metadata) dependencies() ([]Dependency, error) {
	var res []Dependency
	if val, ok := m["dependson"]; ok && strings.TrimSpace(val) != "" {
		addons := strings.Split(val, " ")
		for i := range addons {
			parts := strings.Split(addons[i], ">=")
			if len(parts) == 1 {
				res = append(res, Dependency{Name: parts[0]})
			} else if len(parts) == 2 {
				version, err := strconv.Atoi(parts[1])
				if err != nil {
					return nil, fmt.Errorf("bad dependency version: %s", addons[i])
				}
				res = append(res, Dependency{
					Name:           parts[0],
					MinimumVersion: version,
				})
			} else {
				return nil, fmt.Errorf("invalid dependency: %s", addons[i])
			}
		}
	}
	return res, nil
}
