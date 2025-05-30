package restclient

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func parse(content string) (*RestClient, error) {
	ret := &RestClient{}
	r := bufio.NewScanner(strings.NewReader(strings.TrimSpace(content)))
	if !r.Scan() {
		return nil, errors.New("no command line found")
	}
	ret.method, ret.path, ret.version = parseCommandLine(strings.TrimSpace(r.Text()))
	lineno := 1
	headersBegin := false
	for r.Scan() {
		line := strings.TrimSpace(r.Text())
		lineno++
		if line == "" {
			// end of headers
			break
		}
		if strings.HasPrefix(line, "?") && !headersBegin {
			// This is the cmd extension line
			ret.path += line
			continue
		} else if strings.HasPrefix(line, "&") && !headersBegin {
			ret.path += line
			continue
		}

		headersBegin = true
		key, value := parseHeaderLine(line)
		if key == "" {
			return nil, fmt.Errorf("invalid header line at %q line %d", line, lineno)
		}
		if ret.header == nil {
			ret.header = make(http.Header)
		}
		ret.header.Add(key, value)
	}
	if strings.Contains(ret.path, "?") {
		// split the path and query parameters
		parts := strings.SplitN(ret.path, "?", 2)
		if len(parts) > 1 {
			ret.queryParams = parseParamLine(parts[1])
		}
		ret.path = parts[0] + "?" + ret.queryParams.Encode()
	}

	for r.Scan() {
		line := strings.TrimSpace(r.Text())
		ret.contentLines = append(ret.contentLines, line)
	}
	return ret, nil
}

// parseCommandLine parses http request command line, contains the method, path, and optional version
func parseCommandLine(line string) (method, path, version string) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return "", "", ""
	}
	method = parts[0]
	path = parts[1]
	if len(parts) > 2 {
		version = parts[2]
	}
	return method, path, version
}

// parseHeaderLine parses a single header line into htt.Header.
func parseHeaderLine(line string) (string, string) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	return key, value
}

// parseRequestLine parses a request path into net.URL.
func parseParamLine(line string) url.Values {
	params := url.Values{}
	parts := strings.Split(line, "&")
	for _, part := range parts {
		if part == "" {
			continue
		}
		keyValue := strings.SplitN(part, "=", 2)
		if len(keyValue) == 2 {
			key := strings.TrimSpace(keyValue[0])
			value := strings.TrimSpace(keyValue[1])
			params.Add(key, value)
		} else {
			params.Add(strings.TrimSpace(keyValue[0]), "")
		}
	}
	return params
}
