package files

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
)

func URIToReader(uri string) (io.ReadCloser, error) {
	requestURI, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, err
	}
	if requestURI.Scheme != "file" {
		return nil, fmt.Errorf("unsupported uri scheme: %s", requestURI.Scheme)
	}
	file, err := os.Open(requestURI.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("uri not found in fs: %w", err)
		}
		return nil, err
	}
	return file, nil
}

func PathToFileURI(p string) string {
	return (&url.URL{Scheme: "file", Path: p}).String()
}
