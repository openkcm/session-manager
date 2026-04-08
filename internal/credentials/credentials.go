package credentials

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

const (
	contentType = "Content-Type"
	urlencoded  = "application/x-www-form-urlencoded"
)

type TransportCredentials interface {
	Transport() http.RoundTripper
}

type clientAuthRoundTripper struct {
	clientID     string
	clientSecret string
	next         http.RoundTripper
}

func (rt *clientAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodPost {
		return rt.next.RoundTrip(req)
	}

	if req.Body == nil {
		req.Body = http.NoBody
	}

	switch ct, _, _ := mime.ParseMediaType(req.Header.Get(contentType)); ct {
	case "":
		req.Header.Set(contentType, urlencoded)
		fallthrough
	case urlencoded:
		if err := req.ParseForm(); err != nil {
			return nil, fmt.Errorf("parsing form: %w", err)
		}

		q := req.PostForm
		q.Set("client_id", rt.clientID)
		if rt.clientSecret != "" {
			q.Set("client_secret", rt.clientSecret)
		}

		req.Form = nil
		req.PostForm = nil

		s := strings.NewReader(q.Encode())
		req.Body = io.NopCloser(s)
		req.ContentLength = int64(s.Len())

		snapshot := *s
		req.GetBody = func() (io.ReadCloser, error) {
			r := snapshot
			return io.NopCloser(&r), nil
		}
	}

	return rt.next.RoundTrip(req)
}

type Builder func(clientID string) TransportCredentials
