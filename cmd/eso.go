package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/csmith/addman/common"
	"github.com/csmith/addman/eso"
	"github.com/csmith/config"
	"github.com/mgutz/ansi"
)

var (
	prefixSuccess = ansi.Color("✓", "green")
	prefixWarning = ansi.Color("⚠", "yellow")
	prefixError = ansi.Color("X", "red")
)

type Resolution struct {
	Selected int
	Options  []int
}

func (r *Resolution) covers(addons []eso.FileListEntry) bool {
	for i := range addons {
		found := false
		for j := range r.Options {
			if r.Options[j] == addons[i].Id {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (r *Resolution) selected(matching []eso.FileListEntry) eso.FileListEntry {
	for i := range matching {
		if matching[i].Id == r.Selected {
			return matching[i]
		}
	}

	panic(fmt.Sprintf("No matching resolution found. Selected: %d, available: %v", r.Selected, matching))
}

type EsoConfig struct {
	Path           string
	Checksums      map[string]string
	Resolutions    map[string]*Resolution
	CachedFileList *eso.FileList
	CachedTime     time.Time
}

type Config struct {
	Eso     EsoConfig
	Version int
}

var (
	conf    = &Config{}
	updates []int
)

func main() {
	conf = &Config{}

	c, err := config.Load(conf, config.DirectoryName("addman"))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := c.Save(conf); err != nil {
			panic(err)
		}
	}()

	if conf.Eso.Path == "" {
		promptForDirectory()
	}

	if conf.Eso.Checksums == nil {
		conf.Eso.Checksums = make(map[string]string)
	}
	if conf.Eso.Resolutions == nil {
		conf.Eso.Resolutions = make(map[string]*Resolution)
	}

	if conf.Eso.CachedFileList == nil || conf.Eso.CachedTime.Before(time.Now().Add(-24*time.Hour)) {
		if err := downloadFileList(); err != nil {
			panic(err)
		}
	}

	for i := range os.Args[1:] {
		if v, err := strconv.Atoi(os.Args[i+1]); err == nil {
			updates = append(updates, v)
		}
	}

	checkAgain := true
	for checkAgain {
		checkAddons()

		fmt.Printf("%s %d addons require updating or installing\n", prefixSuccess, len(updates))
		if len(updates) > 0 {
			if err := downloadFileDetails(); err != nil {
				panic(err)
			}
			updates = []int{}
		} else {
			checkAgain = false
		}
	}
}

func promptForDirectory() {
	err := survey.AskOne(&survey.Input{
		Message: "Path to ESO AddOns folder:",
		Default: eso.GuessDirectory(),
		Suggest: func(toComplete string) []string {
			files, _ := filepath.Glob(toComplete + "*")
			return files
		},
	}, &conf.Eso.Path)
	if err != nil {
		panic(err)
	}
}

func downloadFileList() error {
	res, err := http.Get("https://api.mmoui.com/v4/game/ESO/filelist.json")
	if err != nil {
		return err
	}

	defer res.Body.Close()
	list, err := eso.ParseFileList(res.Body)
	if err != nil {
		return err
	}

	conf.Eso.CachedFileList = &list
	conf.Eso.CachedTime = time.Now()
	return nil
}

func downloadFileDetails() error {
	ids := strings.Builder{}
	for i := range updates {
		if ids.Len() > 0 {
			ids.WriteRune(',')
		}
		ids.WriteString(strconv.Itoa(updates[i]))
	}
	res, err := http.Get(fmt.Sprintf("https://api.mmoui.com/v4/game/ESO/filedetails/%s.json", ids.String()))
	if err != nil {
		return err
	}

	defer res.Body.Close()
	details, err := eso.ParseFileDetailsList(res.Body)
	if err != nil {
		return err
	}

	return installUpdates(details)
}

func installUpdates(details eso.FileDetailsList) error {
	for i := range details {
		dirs, err := common.InstallZippedAddonFromUrl(conf.Eso.Path, details[i].DownloadUrl)
		if err != nil {
			fmt.Printf("%s Failed to install '%s': %v\n", prefixError, details[i].Title, err)
			continue
		}

		fmt.Printf("%s Installed '%s' version %s\n", prefixSuccess, details[i].Title, details[i].Version)
		for j := range dirs {
			conf.Eso.Checksums[dirs[j]] = details[i].Checksum
		}
	}
	return nil
}

func checkAddons() {
	addons, err := eso.InstalledAddons(conf.Eso.Path)
	if err != nil {
		panic(err)
	}

	checked := make(map[string]bool)

	for i := range addons {
		if addons[i].Error != nil {
			fmt.Printf("%s Failed to scan addon '%s': %v\n", prefixWarning, addons[i].Name, addons[i].Error)
			continue
		}

		if !checked[addons[i].Name] {
			checkAddon(addons[i].Name)
			checked[addons[i].Name] = true
		}

		for d := range addons[i].DependsOn {
			name := addons[i].DependsOn[d].Name
			if !checked[name] {
				checkAddon(name)
				checked[name] = true
			}
		}
	}
}

func checkAddon(name string) {
	matching := conf.Eso.CachedFileList.ByPath(name)
	switch len(matching) {
	case 0:
		fmt.Printf("%s No downloadable addons found providing '%s' - will not be updated\n", prefixWarning, name)
	case 1:
		checkMatchedAddon(name, matching[0])
	default:
		if res, ok := conf.Eso.Resolutions[name]; ok && res.covers(matching) {
			checkMatchedAddon(name, res.selected(matching))
		} else {
			promptForResolution(name, matching)
		}
	}
}

func promptForResolution(name string, matching []eso.FileListEntry) {
	options := make([]string, len(matching))
	ids := make([]int, len(matching))

	for i := range matching {
		options[i] = fmt.Sprintf(
			"'%s' by %s (%d downloads, last updated %s)",
			matching[i].Title,
			matching[i].Author,
			matching[i].Downloads,
			time.Unix(matching[i].LastUpdate/1000, 0),
		)
		ids[i] = matching[i].Id
	}

	var selected string
	err := survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("Addon %s has multiple providers on eso-ui.com, which would you like to use?", name),
		Options: options,
	}, &selected)
	if err != nil {
		panic(err)
	}

	for i := range options {
		if options[i] == selected {
			conf.Eso.Resolutions[name] = &Resolution{
				Selected: ids[i],
				Options:  ids,
			}
			return
		}
	}
}

func checkMatchedAddon(name string, entry eso.FileListEntry) {
	if cs, ok := conf.Eso.Checksums[name]; !ok || cs != entry.Checksum {
		updates = append(updates, entry.Id)
	}
}
