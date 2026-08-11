package routes

import (
	"net/http"
	"testing"
)

func TestParseRequestedRange(t *testing.T) {
	tests := []struct {
		name       string
		size       int64
		header     string
		wantStart  int64
		wantEnd    int64
		wantStatus int
		wantErr    bool
	}{
		{name: "full response", size: 1000, wantStart: 0, wantEnd: 999, wantStatus: http.StatusOK},
		{name: "explicit range", size: 1000, header: "bytes=100-199", wantStart: 100, wantEnd: 199, wantStatus: http.StatusPartialContent},
		{name: "open ended range", size: 1000, header: "bytes=900-", wantStart: 900, wantEnd: 999, wantStatus: http.StatusPartialContent},
		{name: "suffix range", size: 1000, header: "bytes=-100", wantStart: 900, wantEnd: 999, wantStatus: http.StatusPartialContent},
		{name: "unsatisfiable range", size: 1000, header: "bytes=1000-1001", wantErr: true},
		{name: "multiple ranges unsupported", size: 1000, header: "bytes=0-1,10-11", wantErr: true},
		{name: "empty file", size: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, status, err := parseRequestedRange(tt.size, tt.header)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if start != tt.wantStart || end != tt.wantEnd || status != tt.wantStatus {
				t.Fatalf("got (%d, %d, %d), want (%d, %d, %d)", start, end, status, tt.wantStart, tt.wantEnd, tt.wantStatus)
			}
		})
	}
}
