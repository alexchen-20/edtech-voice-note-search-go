package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestEmbeddingDecodesOpenAICompatibleResponse(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"object":"list","data":[{"object":"embedding","embedding":[0.25,-0.5],"index":0}],"model":"auto"}`)),
		}, nil
	})}
	a := &API{key: "test-key", client: client}

	got, err := a.embedding("hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 0.25 || got[1] != -0.5 {
		t.Fatalf("embedding()=%v, want [0.25 -0.5]", got)
	}
}

func TestDueSoonDrivesEducatorReport(t *testing.T) {
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, deadline string
		want           bool
	}{{"this week", "2026-09-22", true}, {"later", "2026-10-01", false}, {"invalid", "soon", false}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := dueSoon(tc.deadline, now); got != tc.want {
				t.Fatalf("dueSoon(%q)=%v, want %v", tc.deadline, got, tc.want)
			}
		})
	}
}
