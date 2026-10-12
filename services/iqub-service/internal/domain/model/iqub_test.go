package model

import (
	"errors"
	"testing"
	"time"
)

func TestNewIqub(t *testing.T) {
	now := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		iqub    string
		amount  int64
		freq    Frequency
		max     int
		admin   string
		wantErr bool
	}{
		{"valid", "Family Iqub", 50000, Monthly, 10, "m1", false},
		{"empty name", "", 50000, Monthly, 10, "m1", true},
		{"blank name", "   ", 50000, Monthly, 10, "m1", true},
		{"zero amount", "A", 0, Monthly, 10, "m1", true},
		{"negative amount", "A", -5, Monthly, 10, "m1", true},
		{"bad frequency", "A", 100, Frequency("DAILY"), 10, "m1", true},
		{"too few members", "A", 100, Weekly, 1, "m1", true},
		{"too many members", "A", 100, Weekly, 51, "m1", true},
		{"missing admin", "A", 100, Weekly, 5, "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewIqub("id1", tc.iqub, tc.amount, tc.freq, tc.max, tc.admin, now)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				if !errors.Is(err, ErrValidation) {
					t.Fatalf("expected ErrValidation, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Status != StatusOpen {
				t.Errorf("expected OPEN, got %s", got.Status)
			}
			if got.AdminID != "m1" || len(got.MemberIDs) != 1 || got.MemberIDs[0] != "m1" {
				t.Errorf("admin must be the first member, got %+v", got.MemberIDs)
			}
		})
	}
}
