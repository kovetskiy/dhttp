// Copyright 2014-2015 Liu Dong <ddliuhb@gmail.com>.
// Licensed under the MIT license.

package dhttp

import (
	"fmt"

	"bytes"
	"strings"

	"time"

	"io"
	"io/ioutil"
	"sync"

	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"compress/gzip"

	"mime/multipart"
)

const (
	VERSION = "2.0"

	PROXY_HTTP = iota
	PROXY_SOCKS4
	PROXY_SOCKS5
	PROXY_SOCKS4A

	OPT_AUTOREFERER
	OPT_FOLLOWLOCATION
	OPT_CONNECTTIMEOUT
	OPT_CONNECTTIMEOUT_MS
	OPT_MAXREDIRS
	OPT_PROXYTYPE
	OPT_TIMEOUT
	OPT_TIMEOUT_MS
	OPT_COOKIEJAR
	OPT_INTERFACE
	OPT_PROXY
	OPT_REFERER
	OPT_USERAGENT

	OPT_REDIRECT_POLICY
	OPT_PROXY_FUNC
)

// Default options for any clients.
var defaultOptions = map[int]interface{}{
	OPT_FOLLOWLOCATION: true,
	OPT_MAXREDIRS:      10,
	OPT_AUTOREFERER:    true,
	OPT_USERAGENT:      "",
	OPT_COOKIEJAR:      true,
}

// These options affect transport, transport may not be reused if you change any
// of these options during a request.
var transportOptions = []int{
	OPT_CONNECTTIMEOUT,
	OPT_CONNECTTIMEOUT_MS,
	OPT_PROXYTYPE,
	OPT_TIMEOUT,
	OPT_TIMEOUT_MS,
	OPT_INTERFACE,
	OPT_PROXY,
	OPT_PROXY_FUNC,
}

// These options affect cookie jar, jar may not be reused if you change any of
// these options during a request.
var jarOptions = []int{
	OPT_COOKIEJAR,
}

// Thin wrapper of http.Response(can also be used as http.Response).
type Response struct {
	*http.Response
}

// Read response body into a byte slice.
func (respoonse *Response) ReadAll() ([]byte, error) {
	var reader io.ReadCloser
	var err error
	switch respoonse.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(respoonse.Body)
		if err != nil {
			return nil, err
		}
	default:
		reader = respoonse.Body
	}

	defer reader.Close()
	return ioutil.ReadAll(reader)
}

// Read response body into string.
func (respoonse *Response) ToString() (string, error) {
	bytes, err := respoonse.ReadAll()
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// Prepare a request.
func prepareRequest(method string, url string, headers map[string]string,
	body io.Reader, options map[int]interface{}) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)

	if err != nil {
		return nil, err
	}

	// OPT_REFERER
	if referer, ok := options[OPT_REFERER]; ok {
		if refererStr, ok := referer.(string); ok {
			req.Header.Set("Referer", refererStr)
		}
	}

	// OPT_USERAGENT
	if useragent, ok := options[OPT_USERAGENT]; ok {
		if useragentStr, ok := useragent.(string); ok {
			req.Header.Set("User-Agent", useragentStr)
		}
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return req, nil
}

// Prepare a transport.
//
// Handles timemout, proxy and maybe other transport related options here.
func prepareTransport(options map[int]interface{}) (http.RoundTripper, error) {
	transport := &http.Transport{}

	connectTimeoutMS := 0

	if connectTimeoutMS_, ok := options[OPT_CONNECTTIMEOUT_MS]; ok {
		if connectTimeoutMS, ok = connectTimeoutMS_.(int); !ok {
			return nil, fmt.Errorf("OPT_CONNECTTIMEOUT_MS must be int")
		}
	} else if connectTimeout_, ok := options[OPT_CONNECTTIMEOUT]; ok {
		if connectTimeout, ok := connectTimeout_.(int); ok {
			connectTimeoutMS = connectTimeout * 1000
		} else {
			return nil, fmt.Errorf("OPT_CONNECTTIMEOUT must be int")
		}
	}

	timeoutMS := 0

	if timeoutMS_, ok := options[OPT_TIMEOUT_MS]; ok {
		if timeoutMS, ok = timeoutMS_.(int); !ok {
			return nil, fmt.Errorf("OPT_TIMEOUT_MS must be int")
		}
	} else if timeout_, ok := options[OPT_TIMEOUT]; ok {
		if timeout, ok := timeout_.(int); ok {
			timeoutMS = timeout * 1000
		} else {
			return nil, fmt.Errorf("OPT_TIMEOUT must be int")
		}
	}

	// fix connect timeout(important, or it might cause a long time wait during
	//connection)
	if timeoutMS > 0 && (connectTimeoutMS > timeoutMS || connectTimeoutMS == 0) {
		connectTimeoutMS = timeoutMS
	}

	transport.Dial = func(network, addr string) (net.Conn, error) {
		var conn net.Conn
		var err error
		if connectTimeoutMS > 0 {
			conn, err = net.DialTimeout(
				network, addr, time.Duration(connectTimeoutMS)*time.Millisecond,
			)
			if err != nil {
				return nil, err
			}
		} else {
			conn, err = net.Dial(network, addr)
			if err != nil {
				return nil, err
			}
		}

		if timeoutMS > 0 {
			conn.SetDeadline(
				time.Now().Add(time.Duration(timeoutMS) * time.Millisecond),
			)
		}

		return conn, nil
	}

	// proxy
	if proxyFunc_, ok := options[OPT_PROXY_FUNC]; ok {
		if proxyFunc, ok := proxyFunc_.(func(*http.Request) (int, string, error)); ok {
			transport.Proxy = func(req *http.Request) (*url.URL, error) {
				proxyType, uri, err := proxyFunc(req)
				if err != nil {
					return nil, err
				}

				if proxyType != PROXY_HTTP {
					return nil, fmt.Errorf("only PROXY_HTTP is currently supported")
				}

				uri = "http://" + uri

				parsedURL, err := url.Parse(uri)

				if err != nil {
					return nil, err
				}

				return parsedURL, nil
			}
		} else {
			return nil, fmt.Errorf("OPT_PROXY_FUNC is not a desired function")
		}
	} else {
		var proxytype int
		if proxytype_, ok := options[OPT_PROXYTYPE]; ok {
			if proxytype, ok = proxytype_.(int); !ok || proxytype != PROXY_HTTP {
				return nil, fmt.Errorf(
					"OPT_PROXYTYPE must be int, " +
						"and only PROXY_HTTP is currently supported",
				)
			}
		}

		var proxy string
		if proxy_, ok := options[OPT_PROXY]; ok {
			if proxy, ok = proxy_.(string); !ok {
				return nil, fmt.Errorf("OPT_PROXY must be string")
			}
			proxy = "http://" + proxy
			proxyUrl, err := url.Parse(proxy)
			if err != nil {
				return nil, err
			}
			transport.Proxy = http.ProxyURL(proxyUrl)
		}
	}

	return transport, nil
}

// Prepare a redirect policy.
func prepareRedirect(
	options map[int]interface{},
) (func(req *http.Request, via []*http.Request) error, error) {
	var redirectPolicy func(req *http.Request, via []*http.Request) error

	if redirectPolicyOption, ok := options[OPT_REDIRECT_POLICY]; ok {
		if redirectPolicy, ok = redirectPolicyOption.(func(*http.Request, []*http.Request) error); !ok {
			return nil, fmt.Errorf("OPT_REDIRECT_POLICY is not a desired function")
		}
	} else {
		var followlocation bool
		if followlocation_, ok := options[OPT_FOLLOWLOCATION]; ok {
			if followlocation, ok = followlocation_.(bool); !ok {
				return nil, fmt.Errorf("OPT_FOLLOWLOCATION must be bool")
			}
		}

		var maxredirs int
		if maxredirs_, ok := options[OPT_MAXREDIRS]; ok {
			if maxredirs, ok = maxredirs_.(int); !ok {
				return nil, fmt.Errorf("OPT_MAXREDIRS must be int")
			}
		}

		redirectPolicy = func(req *http.Request, via []*http.Request) error {
			// no follow
			if !followlocation || maxredirs <= 0 {
				return &Error{
					Code:    ERR_REDIRECT_POLICY,
					Message: fmt.Sprintf("redirect not allowed"),
				}
			}

			if len(via) >= maxredirs {
				return &Error{
					Code:    ERR_REDIRECT_POLICY,
					Message: fmt.Sprintf("stopped after %d redirects", len(via)),
				}
			}

			last := via[len(via)-1]
			// keep necessary headers
			// TODO: pass all headers or add other headers?
			if useragent := last.Header.Get("User-Agent"); useragent != "" {
				req.Header.Set("User-Agent", useragent)
			}

			return nil
		}
	}

	return redirectPolicy, nil
}

// Prepare a cookie jar.
func prepareJar(options map[int]interface{}) (http.CookieJar, error) {
	var jar http.CookieJar
	var err error
	if optCookieJar_, ok := options[OPT_COOKIEJAR]; ok {
		// is bool
		if optCookieJar, ok := optCookieJar_.(bool); ok {
			// default jar
			if optCookieJar {
				// TODO: PublicSuffixList
				jar, err = cookiejar.New(nil)
				if err != nil {
					return nil, err
				}
			}
		} else if optCookieJar, ok := optCookieJar_.(http.CookieJar); ok {
			jar = optCookieJar
		} else {
			return nil, fmt.Errorf("invalid cookiejar")
		}
	}

	return jar, nil
}

// Create an HTTP client.
func NewClient() *Client {
	c := &Client{
		reuseTransport: true,
		reuseJar:       true,
	}

	return c
}

// Powerful and easy to use HTTP client.
type Client struct {
	// Default options of this client.
	Options map[int]interface{}

	// Default headers of this client.
	Headers map[string]string

	// Options of current request.
	oneTimeOptions map[int]interface{}

	// Headers of current request.
	oneTimeHeaders map[string]string

	// Cookies of current request.
	oneTimeCookies []*http.Cookie

	// Global transport of this client, might be shared between different
	// requests.
	transport http.RoundTripper

	// Global cookie jar of this client, might be shared between different
	// requests.
	jar http.CookieJar

	// Whether current request should reuse the transport or not.
	reuseTransport bool

	// Whether current request should reuse the cookie jar or not.
	reuseJar bool

	// Make requests of one client concurrent safe.
	lock *sync.Mutex
}

// Set default options and headers.
func (client *Client) Defaults(
	options map[int]interface{}, headers map[string]string,
) *Client {
	// merge options
	if client.Options == nil {
		client.Options = options
	} else {
		for name, value := range options {
			client.Options[name] = value
		}
	}

	// merge headers
	if client.Headers == nil {
		client.Headers = headers
	} else {
		for name, value := range headers {
			client.Headers[name] = value
		}
	}

	return client
}

// Begin marks the begining of a request, it's necessary for concurrent
// requests.
func (client *Client) Begin() *Client {
	if client.lock == nil {
		client.lock = new(sync.Mutex)
	}
	client.lock.Lock()

	return client
}

// Reset the client state so that other requests can begin.
func (client *Client) reset() {
	client.oneTimeOptions = nil
	client.oneTimeHeaders = nil
	client.oneTimeCookies = nil
	client.reuseTransport = true
	client.reuseJar = true

	// nil means the Begin has not been called, asume requests are not
	// concurrent.
	if client.lock != nil {
		client.lock.Unlock()
	}
}

// Temporarily specify an option of the current request.
func (client *Client) WithOption(option int, value interface{}) *Client {
	if client.oneTimeOptions == nil {
		client.oneTimeOptions = make(map[int]interface{})
	}
	client.oneTimeOptions[option] = value

	// Conditions we cann't reuse the transport.
	if !hasOption(option, transportOptions) {
		client.reuseTransport = false
	}

	// Conditions we cann't reuse the cookie jar.
	if !hasOption(option, jarOptions) {
		client.reuseJar = false
	}

	return client
}

// Temporarily specify multiple options of the current request.
func (client *Client) WithOptions(options map[int]interface{}) *Client {
	for option, value := range options {
		client.WithOption(option, value)
	}

	return client
}

// Temporarily specify a header of the current request.
func (client *Client) WithHeader(option string, value string) *Client {
	if client.oneTimeHeaders == nil {
		client.oneTimeHeaders = make(map[string]string)
	}
	client.oneTimeHeaders[option] = value

	return client
}

// Temporarily specify multiple headers of the current request.
func (client *Client) WithHeaders(headers map[string]string) *Client {
	for header, value := range headers {
		client.WithHeader(header, value)
	}

	return client
}

// Specify cookies of the current request.
func (client *Client) WithCookie(cookies ...*http.Cookie) *Client {
	client.oneTimeCookies = append(client.oneTimeCookies, cookies...)

	return client
}

// Start a request, and get the response.
//
// Usually we just need the Get and Post method.
func (client *Client) Do(
	method string, url string, headers map[string]string, body io.Reader,
) (*Response, error) {
	var (
		transport http.RoundTripper
		jar       http.CookieJar
		err       error
	)

	var (
		options = mergeOptions(
			defaultOptions, client.Options, client.oneTimeOptions,
		)

		cookies = client.oneTimeCookies
	)

	headers = mergeHeaders(client.Headers, client.oneTimeHeaders, headers)

	// transport
	if client.transport == nil || !client.reuseTransport {
		transport, err = prepareTransport(options)
		if err != nil {
			client.reset()
			return nil, err
		}

		if client.reuseTransport {
			client.transport = transport
		}
	} else {
		transport = client.transport
	}

	// jar
	if client.jar == nil || !client.reuseJar {
		jar, err = prepareJar(options)
		if err != nil {
			client.reset()
			return nil, err
		}

		if client.reuseJar {
			client.jar = jar
		}
	} else {
		jar = client.jar
	}

	// release lock
	client.reset()

	redirect, err := prepareRedirect(options)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Transport:     transport,
		CheckRedirect: redirect,
		Jar:           jar,
	}

	req, err := prepareRequest(method, url, headers, body, options)
	if err != nil {
		return nil, err
	}

	if jar != nil {
		jar.SetCookies(req.URL, cookies)
	} else {
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
	}

	res, err := httpClient.Do(req)

	return &Response{res}, err
}

// The GET request
func (client *Client) Get(
	url string, params url.Values,
) (*Response, error) {
	url = addQuery(url, params)

	return client.Do("GET", url, nil, nil)
}

// The POST request
//
// With multipart set to true, the request will be encoded as
// "multipart/form-data".
//
// If any of the params key starts with "@", it is considered as a form file
// (similar to CURL but different).
func (client *Client) Post(
	url string, params url.Values,
) (*Response, error) {
	// Post with files should be sent as multipart.
	if checkParamFile(params) {
		return client.PostMultipart(url, params)
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	body := strings.NewReader(params.Encode())

	return client.Do("POST", url, headers, body)
}

// Post with the request encoded as "multipart/form-data".
func (client *Client) PostMultipart(
	url string, params url.Values,
) (*Response, error) {

	var (
		body   = &bytes.Buffer{}
		writer = multipart.NewWriter(body)
	)

	for name, values := range params {
		if name[0] == '@' {
			err := addFormFile(writer, name[1:], values[0])
			if err != nil {
				return nil, err
			}
		} else {
			for _, value := range values {
				writer.WriteField(name, value)
			}
		}
	}

	headers := map[string]string{
		"Content-Type": writer.FormDataContentType(),
	}

	err := writer.Close()
	if err != nil {
		return nil, err
	}

	return client.Do("POST", url, headers, body)
}

// Get cookies of the client jar.
func (client *Client) Cookies(uri string) []*http.Cookie {
	if client.jar != nil {
		parsedURL, _ := url.Parse(uri)

		return client.jar.Cookies(parsedURL)
	}

	return nil
}

// Get cookie values(k-v map) of the client jar.
func (client *Client) CookieValues(url string) map[string]string {
	cookies := map[string]string{}

	for _, cookie := range client.Cookies(url) {
		cookies[cookie.Name] = cookie.Value
	}

	return cookies
}

// Get cookie value of a specified cookie name.
func (client *Client) CookieValue(url string, key string) string {
	for _, cookie := range client.Cookies(url) {
		if cookie.Name == key {
			return cookie.Value
		}
	}

	return ""
}
