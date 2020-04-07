package endpoints

import "testing"

func TestNewHttpRequestFromEndpoint(t *testing.T) {
	e := &SingleEndpoint{
		Protocol:    "https",
		HostAndPort: "abc.123:8080",
	}

	r := newHttpRequestFromEndpoint(e, "/abc", "rgw=node1")
	if r.URL.Scheme != e.Protocol {
		t.Errorf("expected %s partitions, got %s", e.Protocol, r.URL.Scheme)
	}
	if r.URL.Host != e.HostAndPort {
		t.Errorf("expected %s partitions, got %s", e.HostAndPort, r.URL.Host)
	}
	if r.URL.Path != "/abc" {
		t.Errorf("expected %s partitions, got %s", "/abc", r.URL.Path)
	}
}

func TestNewHttpRequestFromURL(t *testing.T) {
	URL := "https://abc.123:8080"

	r, err := newHttpRequestFromURL(URL, "/abc", "rgw=node1")
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
	if r.URL.Scheme != "https" {
		t.Errorf("expected %s partitions, got %s", "https", r.URL.Scheme)
	}
	if r.URL.Host != "abc.123:8080" {
		t.Errorf("expected %s partitions, got %s", "abc.123:8080", r.URL.Host)
	}
	if r.URL.Path != "/abc" {
		t.Errorf("expected %s partitions, got %s", "/abc", r.URL.Path)
	}
}
