package dhttp

import (
	"testing"
)

func TestDefaultClient(t *testing.T) {
	res, err := Get("http://httpbin.org/get", nil)

	if err != nil {
		t.Error("get failed", err)
	}

	if res.StatusCode != 200 {
		t.Error("Status Code not 200")
	}
}
