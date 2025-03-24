package main

import (
	"testing"
)

func TestParsePids(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		want    []int
		wantErr bool
	}{
		{
			name:    "Single PID",
			input:   "1234",
			want:    []int{1234},
			wantErr: false,
		},
		{
			name:    "Multiple PIDs",
			input:   "1234,5678,9012",
			want:    []int{1234, 5678, 9012},
			wantErr: false,
		},
		{
			name:    "With spaces",
			input:   " 1234 , 5678 ",
			want:    []int{1234, 5678},
			wantErr: false,
		},
		{
			name:    "Invalid PID",
			input:   "1234,abc",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Negative PID",
			input:   "1234,-1",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Zero PID",
			input:   "0",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePids(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("parsePids() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if tc.wantErr {
				return
			}
			if len(got) != len(tc.want) {
				t.Errorf("parsePids() got %v, want %v", got, tc.want)
				return
			}
			for i, pid := range got {
				if pid != tc.want[i] {
					t.Errorf("parsePids() got %v, want %v", got, tc.want)
					break
				}
			}
		})
	}
}

func TestCalculateBudget(t *testing.T) {
	testCases := []struct {
		name     string
		totalRSS int64
		percent  int
		maxBytes int64
		want     int64
	}{
		{
			name:     "30 percent",
			totalRSS: 100 * 1024 * 1024,
			percent:  30,
			maxBytes: 0,
			want:     30 * 1024 * 1024,
		},
		{
			name:     "With max bytes lower than percent",
			totalRSS: 100 * 1024 * 1024,
			percent:  30,
			maxBytes: 20 * 1024 * 1024,
			want:     20 * 1024 * 1024,
		},
		{
			name:     "With max bytes higher than percent",
			totalRSS: 100 * 1024 * 1024,
			percent:  30,
			maxBytes: 40 * 1024 * 1024,
			want:     30 * 1024 * 1024,
		},
		{
			name:     "Invalid percent defaults to 30",
			totalRSS: 100 * 1024 * 1024,
			percent:  -10,
			maxBytes: 0,
			want:     30 * 1024 * 1024,
		},
		{
			name:     "Percent > 100 defaults to 30",
			totalRSS: 100 * 1024 * 1024,
			percent:  110,
			maxBytes: 0,
			want:     30 * 1024 * 1024,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := calculateBudget(tc.totalRSS, tc.percent, tc.maxBytes)
			if got != tc.want {
				t.Errorf("calculateBudget() = %v, want %v", got, tc.want)
			}
		})
	}
}
