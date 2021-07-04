package common

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// InstallZippedAddonFromUrl downloads a ZIP file from the given URL and deploys it to the target directory, returning a
// slice of top-level folder names that were created.
func InstallZippedAddonFromUrl(target, url string) ([]string, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	return InstallZippedAddon(target, res.Body)
}

// InstallZippedAddon reads a ZIP file from the given reader and deploys it to the target directory, returning a
// slice of top-level folder names that were created.
func InstallZippedAddon(target string, r io.Reader) ([]string, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}

	dirs := make(map[string]bool)

	for i := range reader.File {
		err := func(f *zip.File) error {
			parts := strings.Split(f.Name, "/")
			dirs[parts[0]] = true

			target := filepath.Join(target, f.Name)
			if f.FileInfo().IsDir() {
				return os.MkdirAll(target, os.FileMode(0755))
			} else {
				in, err := f.Open()
				if err != nil {
					return err
				}
				defer in.Close()

				_ = os.MkdirAll(filepath.Dir(target), os.FileMode(0755))
				out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
				if err != nil {
					return err
				}
				defer out.Close()

				if _, err := io.Copy(out, in); err != nil {
					return err
				}

				return nil
			}
		}(reader.File[i])
		if err != nil {
			return nil, err
		}
	}

	var dirSlice []string
	for d := range dirs {
		dirSlice = append(dirSlice, d)
	}
	return dirSlice, nil
}
