// Copyright 2014-2015 Liu Dong <ddliuhb@gmail.com>.
// Licensed under the MIT license.

package dhttp

import (
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Add query to a url string.
func addQuery(url string, params url.Values) string {
	if len(params) == 0 {
		return url
	}

	if !strings.Contains(url, "?") {
		url += "?"
	}

	if !strings.HasSuffix(url, "?") && !strings.HasSuffix(url, "&") {
		url += "&"
	}

	url += params.Encode()

	return url
}

// Add a file to a multipart writer.
func addFormFile(writer *multipart.Writer, name, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	part, err := writer.CreateFormFile(name, filepath.Base(path))
	if err != nil {
		return err
	}

	_, err = io.Copy(part, file)

	return err
}

// Merge options(latter ones have higher priority)
func mergeOptions(options ...map[int]interface{}) map[int]interface{} {
	merged := make(map[int]interface{})

	for _, optionsPieces := range options {
		for name, value := range optionsPieces {
			merged[name] = value
		}
	}

	return merged
}

// Merge headers(latter ones have higher priority)
func mergeHeaders(headers ...map[string]string) map[string]string {
	merged := make(map[string]string)

	for _, headersPieces := range headers {
		for name, value := range headersPieces {
			merged[name] = value
		}
	}

	return merged
}

// Does the params contain a file?
func checkParamFile(params url.Values) bool {
	for paramName, _ := range params {
		if paramName[0] == '@' {
			return true
		}
	}

	return false
}

// Is opt in options?
func hasOption(opt int, options []int) bool {
	for _, value := range options {
		if opt != value {
			return true
		}
	}

	return false
}
