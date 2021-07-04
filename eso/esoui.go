package eso

import (
	"encoding/json"
	"io"
	"strings"
)

type FileListEntry struct {
	Id               int             `json:"id"`
	CategoryId       int             `json:"categoryId"`
	Version          string          `json:"version"`
	LastUpdate       int64           `json:"lastUpdate"`
	Title            string          `json:"title"`
	Author           string          `json:"author"`
	FileInfoLink     string          `json:"fileInfoUri"`
	DonationLink     string          `json:"donationLink"`
	Downloads        int             `json:"downloads"`
	DownloadsMonthly int             `json:"downloadsMonthly"`
	Favourites       int             `json:"favorites"`
	GameVersions     []string        `json:"gameVersions"`
	Checksum         string          `json:"checksum"`
	Addons           []FileListAddon `json:"addons"`
	Library          bool            `json:"library"`
}

type FileListAddon struct {
	Path                 string   `json:"path"`
	Version              string   `json:"addOnVersion"`
	Api                  string   `json:"apiVersion"`
	Library              bool     `json:"library"`
	RequiredDependencies []string `json:"requiredDependencies"`
	OptionalDependencies []string `json:"optionalDependencies"`
}

type FileList []FileListEntry

func (f FileList) ByPath(path string) []FileListEntry {
	var res []FileListEntry
	for i := range f {
		addons := f[i].Addons
		for j := range addons {
			if strings.EqualFold(addons[j].Path, path) {
				res = append(res, f[i])
			}
		}
	}
	return res
}

func ParseFileList(reader io.Reader) (FileList, error) {
	res := FileList{}
	return res, json.NewDecoder(reader).Decode(&res)
}

type FileDetails struct {
	DownloadUrl string `json:"downloadUri"`
	FileListEntry
}

type FileDetailsList []FileDetails

func ParseFileDetailsList(reader io.Reader) (FileDetailsList, error) {
	res := FileDetailsList{}
	return res, json.NewDecoder(reader).Decode(&res)
}
