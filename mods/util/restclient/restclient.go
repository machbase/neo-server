package restclient

import (
	"encoding/hex"
	"encoding/json"
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

	result *RestResult
}

func (rc *RestClient) RoundTrip(req *http.Request) (*http.Response, error) {
	rsp, err := rc.Transport.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("restClient RoundTrip error: %w", err)
	}

	rr := &RestResult{}
	if err := rr.Load(rsp); err != nil {
		return nil, err
	}
	rc.result = rr
	return rsp, nil
}

func (rc *RestClient) Do() *RestResult {
	var client = &http.Client{Transport: rc}
	var payload io.Reader
	if rc.contentLines != nil && len(rc.contentLines) > 0 {
		payload = strings.NewReader(strings.Join(rc.contentLines, "\n"))
	}

	req, err := http.NewRequest(rc.method, rc.path, payload)
	if err != nil {
		return &RestResult{Err: err}
	}
	req.Header = rc.header
	if rc.version != "" {
		req.Proto = rc.version
	}

	rsp, err := client.Do(req)
	if err != nil {
		return &RestResult{Err: err}
	}
	defer rsp.Body.Close()

	return rc.result
}

type RestResult struct {
	StatusLine      string   `json:"statusLine"`
	Header          []Header `json:"header"`
	Body            string   `json:"body,omitempty"`
	ContentType     string   `json:"contentType,omitempty"`
	ContentEncoding string   `json:"contentEncoding,omitempty"`
	Dump            string   `json:"dump,omitempty"`
	Err             error    `json:"error,omitempty"`
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (h *Header) String() string {
	return fmt.Sprintf("%s: %s", h.Name, h.Value)
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

func (rr *RestResult) Load(r *http.Response) error {
	// Get content type without charset, if any
	if ct := r.Header.Get("Content-Type"); ct != "" {
		parts := strings.SplitN(ct, ";", 2)
		if len(parts) > 0 {
			rr.ContentType = strings.TrimSpace(parts[0])
		}
	}
	rr.ContentEncoding = r.Header.Get("Content-Encoding")

	w := &strings.Builder{}
	if err := rr.loadStatusLine(w, r); err != nil {
		return fmt.Errorf("error loading status line: %w", err)
	}
	if err := rr.loadHeader(w, r); err != nil {
		return fmt.Errorf("error loading header: %w", err)
	}
	if err := rr.loadBody(w, r); err != nil {
		return fmt.Errorf("error dumping response body: %w", err)
	}
	rr.Dump = w.String()
	return nil
}

func (rr *RestResult) loadStatusLine(w io.Writer, r *http.Response) error {
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

	rr.StatusLine = fmt.Sprintf("HTTP/%d.%d %03d %s", r.ProtoMajor, r.ProtoMinor, r.StatusCode, text)
	if _, err := fmt.Fprintf(w, "%s\r\n", rr.StatusLine); err != nil {
		return err
	}
	return nil
}

func (rr *RestResult) loadHeader(w io.Writer, r *http.Response) error {
	// Header
	keys := []string{}
	for k := range r.Header {
		keys = append(keys, k)
	}
	// Sort keys for consistent output
	slices.Sort(keys)
	// Write each header line
	for _, k := range keys {
		for _, v := range r.Header.Values(k) {
			rr.Header = append(rr.Header, Header{Name: k, Value: v})
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

func (rr *RestResult) loadBody(w io.Writer, r *http.Response) error {
	if len(rr.ContentType) == 0 {
		return nil
	}

	out := &strings.Builder{}
	if rr.ContentEncoding == "" {
		if rr.ContentType == "application/json" {
			dec := json.NewDecoder(r.Body)
			var m any
			if err := dec.Decode(&m); err == nil {
				// If the body is valid JSON, we can pretty-print it.
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				if err := enc.Encode(m); err != nil {
					return fmt.Errorf("error encoding JSON: %w", err)
				}
				rr.Body = out.String()
			} else {
				// If the body is not valid JSON, we just dump it as is.
				rr.Body = "Invalid JSON: " + err.Error()
			}
		} else {
			_, err := io.Copy(out, r.Body)
			if err != nil {
				return fmt.Errorf("error reading response body: %w", err)
			}
			rr.Body = out.String()
		}
	} else {
		d := hex.Dumper(out)
		if _, err := io.Copy(d, r.Body); err != nil {
			return fmt.Errorf("error reading response body: %w", err)
		}
		d.Close()
		rr.Body = out.String()
	}
	_, err := fmt.Fprintf(w, rr.Body)
	return err
}
