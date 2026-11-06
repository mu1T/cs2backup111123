package steam

import (
	"testing"
)

func TestAccountIDToSteam64(t *testing.T) {
	tests := []struct {
		accountID uint32
		want      uint64
	}{
		{0, 76561197960265728},
		{123, 76561197960265851},
		{123456789, 76561198083722517},
		{666000333, 76561198626266061},
	}

	for _, tt := range tests {
		got := AccountIDToSteam64(tt.accountID)
		if got != tt.want {
			t.Errorf("AccountIDToSteam64(%d) = %d, want %d", tt.accountID, got, tt.want)
		}
	}
}

func TestSteam64ToAccountID(t *testing.T) {
	tests := []struct {
		steam64 uint64
		want    uint32
	}{
		{76561197960265728, 0},
		{76561197960265851, 123},
		{76561198083722517, 123456789},
		{76561198626266061, 666000333},
		{100, 0}, // Below base
	}

	for _, tt := range tests {
		got := Steam64ToAccountID(tt.steam64)
		if got != tt.want {
			t.Errorf("Steam64ToAccountID(%d) = %d, want %d", tt.steam64, got, tt.want)
		}
	}
}

func TestStringConversions(t *testing.T) {
	s64, err := AccountIDStringToSteam64("123456789")
	if err != nil || s64 != 76561198083722517 {
		t.Fatalf("AccountIDStringToSteam64 failed: got %d, err %v", s64, err)
	}

	_, err = AccountIDStringToSteam64("invalid")
	if err == nil {
		t.Fatal("expected error for invalid account ID string")
	}

	aid, err := Steam64StringToAccountID("76561198083722517")
	if err != nil || aid != 123456789 {
		t.Fatalf("Steam64StringToAccountID failed: got %d, err %v", aid, err)
	}

	_, err = Steam64StringToAccountID("100")
	if err == nil {
		t.Fatal("expected error for below-base steam64")
	}
}
