package restclient

import "net/http"

func Parse(content string) (*RestClient, error) {
	return parse(content)
}

type RestClient struct {
	method       string      // HTTP method, e.g., "GET", "POST"
	path         string      // Request path, e.g., "/api/data"
	version      string      // HTTP version, e.g., "HTTP/1.1"
	header       http.Header // HTTP headers
	contentLines []string
}

func (rc *RestClient) Do(url string) (*RestResult, error) {
	return &RestResult{}, nil
}

type RestResult struct {
}
