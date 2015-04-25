package dhttp

import (
	"io"
	"net/http"
	"net/url"
)

var client *Client

// Set default options and headers.
func Defaults(
	options map[int]interface{}, headers map[string]string,
) *Client {
	return client.Defaults(options, headers)
}

// Begin marks the begining of a request, it's necessary for concurrent
// requests.
func Begin() *Client {
	return client.Begin()
}

// Temporarily specify an option of the current request.
func WithOption(option int, value interface{}) *Client {
	return client.WithOption(option, value)
}

// Temporarily specify multiple options of the current request.
func WithOptions(options map[int]interface{}) *Client {
	return client.WithOptions(options)
}

// Temporarily specify a header of the current request.
func WithHeader(option string, value string) *Client {
	return client.WithHeader(option, value)
}

// Temporarily specify multiple headers of the current request.
func WithHeaders(headers map[string]string) *Client {
	return client.WithHeaders(headers)
}

// Specify cookies of the current request.
func WithCookie(cookies ...*http.Cookie) *Client {
	return client.WithCookie(cookies...)
}

// Start a request, and get the response.
//
// Usually we just need the Get and Post method.
func Do(
	method string, url string, headers map[string]string, body io.Reader,
) (*Response, error) {
	return client.Do(method, url, headers, body)
}

// The GET request
func Get(
	url string, params url.Values,
) (*Response, error) {
	return client.Get(url, params)
}

// The POST request
//
// With multipart set to true, the request will be encoded as
// "multipart/form-data".
//
// If any of the params key starts with "@", it is considered as a form file
// (similar to CURL but different).
func Post(
	url string, params url.Values,
) (*Response, error) {
	return client.Post(url, params)
}

// Post with the request encoded as "multipart/form-data".
func PostMultipart(
	url string, params url.Values,
) (*Response, error) {
	return client.PostMultipart(url, params)
}

// Get cookies of the client jar.
func Cookies(uri string) []*http.Cookie {
	return client.Cookies(uri)
}

// Get cookie values(k-v map) of the client jar.
func CookieValues(url string) map[string]string {
	return client.CookieValues(url)
}

// Get cookie value of a specified cookie name.
func CookieValue(url string, key string) string {
	return client.CookieValue(url, key)
}

func init() {
	client = new(Client)
}
