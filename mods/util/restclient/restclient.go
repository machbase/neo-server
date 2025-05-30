package restclient

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

func Parse(content string) (*RestClient, error) {
	ret, err := parse(content)
	if err != nil {
		return nil, fmt.Errorf("restClient parse error: %w", err)
	}

	ret.Transport = &http.Transport{
		Proxy:              http.ProxyFromEnvironment,
		DisableCompression: true,
	}
	return ret, err
}

type RestClient struct {
	*http.Transport             // Embed the default HTTP round tripper
	method          string      // HTTP method, e.g., "GET", "POST"
	path            string      // Request path, e.g., "/api/data"
	queryParams     url.Values  // Query parameters, e.g., "key=value&key2=value2"
	version         string      // HTTP version, e.g., "HTTP/1.1"
	header          http.Header // HTTP headers
	contentLines    []string

	rspContentType     string
	rspContentEncoding string
	rspDump            string
}

func (rc *RestClient) RoundTrip(req *http.Request) (*http.Response, error) {
	rsp, err := rc.Transport.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("restClient RoundTrip error: %w", err)
	}

	rc.rspContentType = rsp.Header.Get("Content-Type")
	rc.rspContentEncoding = rsp.Header.Get("Content-Encoding")

	if len(rc.rspContentType) > 0 {
		w := &strings.Builder{}
		if err := dumpHeader(w, rsp); err != nil {
			return nil, err
		}
		if err := dumpBody(w, rsp, rc.rspContentEncoding); err != nil {
			return nil, fmt.Errorf("error dumping response body: %w", err)
		}
		rc.rspDump = w.String()
	}
	return rsp, nil
}

func (rc *RestClient) Do() *RestResult {
	var client = &http.Client{Transport: rc}
	var payload io.Reader
	if rc.contentLines != nil && len(rc.contentLines) > 0 {
		payload = strings.NewReader(strings.Join(rc.contentLines, "\n"))
	}

	ret := &RestResult{}
	req, err := http.NewRequest(rc.method, rc.path, payload)
	if err != nil {
		ret.Err = err
		return ret
	}
	req.Header = rc.header
	if rc.version != "" {
		req.Proto = rc.version
	}

	rsp, err := client.Do(req)
	if err != nil {
		ret.Err = err
		return ret
	}
	defer rsp.Body.Close()

	ret.Dump = rc.rspDump
	ret.ContentType = rc.rspContentType

	return ret
}

type RestResult struct {
	ContentType string `json:"contentType"`
	Dump        string `json:"dump"`
	Err         error  `json:"error,omitempty"`
}

func (rr *RestResult) String() string {
	if rr.Err != nil {
		return rr.Err.Error()
	}
	return rr.Dump
}

// json() returns the JSON body of the response if it exists.
// It is for convenience to extract the JSON part from the response data,
// for testing purposes.
func (rr *RestResult) json() string {
	if rr.Err != nil || rr.Dump == "" {
		return ""
	}
	lines := strings.Split(rr.Dump, "\n")
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			if i+1 < len(lines) {
				return strings.Join(lines[i+1:], "\n")
			}
		}
	}
	return ""
}

func dumpHeader(w io.Writer, r *http.Response) error {
	// Status line
	text := r.Status
	if text == "" {
		text = http.StatusText(r.StatusCode)
		if text == "" {
			text = "status code " + strconv.Itoa(r.StatusCode)
		}
	} else {
		// Just to reduce stutter, if user set r.Status to "200 OK" and StatusCode to 200.
		// Not important.
		text = strings.TrimPrefix(text, strconv.Itoa(r.StatusCode)+" ")
	}

	if _, err := fmt.Fprintf(w, "HTTP/%d.%d %03d %s\r\n", r.ProtoMajor, r.ProtoMinor, r.StatusCode, text); err != nil {
		return err
	}

	// Header
	keys := []string{}
	for k := range r.Header {
		keys = append(keys, k)
	}
	// Sort keys for consistent output
	slices.Sort(keys)
	// Write each header line
	for _, k := range keys {
		vv := r.Header.Values(k)
		// Write each header line
		for _, v := range vv {
			if _, err := fmt.Fprintf(w, "%s: %s\r\n", k, v); err != nil {
				return err
			}
		}
	}

	// End-of-header
	if _, err := io.WriteString(w, "\r\n"); err != nil {
		return err
	}
	return nil
}

func dumpBody(w io.Writer, r *http.Response, contentEncoding string) error {
	if contentEncoding == "" {
		_, err := io.Copy(w, r.Body)
		return err
	}

	d := hex.Dumper(w)
	_, err := io.Copy(d, r.Body)
	d.Close()
	return err
}
