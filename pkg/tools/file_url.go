package tools

import (
	"net/url"
	"path/filepath"
	"strings"
)

// FileURL converts an absolute filesystem path to a file URL.
func FileURL(path string) string {
	if volume := filepath.VolumeName(path); volume != "" {
		if strings.HasPrefix(volume, `\\`) {
			path = filepath.ToSlash(path[2:])
			host, urlPath, found := strings.Cut(path, "/")
			if !found {
				urlPath = "/"
			} else {
				urlPath = "/" + urlPath
			}
			return (&url.URL{Scheme: "file", Host: host, Path: urlPath}).String()
		}
		path = "/" + filepath.ToSlash(path)
	} else {
		path = filepath.ToSlash(path)
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}
