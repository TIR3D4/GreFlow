package console

import (
	"net/http"
	"net/url"
)

func systemHTTPProxy(_ *http.Request) (*url.URL, error) { return nil, nil }
