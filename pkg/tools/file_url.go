package tools

import (
	"net/url"
	"path/filepath"
	"strings"
)

// FileURL converts an absolute filesystem path to a file URL.
func FileURL(path string) string {
	return pathToFileURL(path, filepath.VolumeName(path))
}

func pathToFileURL(path, volume string) string {
	if volume != "" {
		if strings.HasPrefix(volume, `\\`) {
			path = strings.ReplaceAll(path[2:], `\`, "/")
			host, urlPath, found := strings.Cut(path, "/")
			if !found {
				urlPath = "/"
			} else {
				urlPath = "/" + urlPath
			}
			return (&url.URL{Scheme: "file", Host: host, Path: urlPath}).String()
		}
		path = "/" + strings.ReplaceAll(path, `\`, "/")
	} else {
		path = filepath.ToSlash(path)
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}
