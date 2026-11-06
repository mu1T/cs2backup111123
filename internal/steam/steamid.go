package steam

import (
	"fmt"
	"strconv"
)

// SteamUniverseBase represents the 64-bit Steam ID base offset (Steam Universe base for individual accounts).
const SteamUniverseBase uint64 = 76561197960265728
const steam64Base uint64 = SteamUniverseBase

// AccountIDToSteamID64 converts a 32-bit Steam Account ID to a 64-bit SteamID64.
func AccountIDToSteamID64(accountID uint32) uint64 {
	return uint64(accountID) + steam64Base
}

// SteamID64ToAccountID converts a 64-bit SteamID64 to a 32-bit Steam Account ID.
func SteamID64ToAccountID(steamID64 uint64) uint32 {
	if steamID64 < steam64Base {
		return 0
	}
	return uint32(steamID64 - steam64Base)
}

// AccountIDStringToSteamID64 parses an Account ID string and converts it to SteamID64.
func AccountIDStringToSteamID64(accountIDStr string) (uint64, error) {
	aid, err := strconv.ParseUint(accountIDStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid account ID %s: %w", accountIDStr, err)
	}
	return AccountIDToSteamID64(uint32(aid)), nil
}

// SteamID64StringToAccountID parses a SteamID64 string and converts it to Account ID.
func SteamID64StringToAccountID(steamID64Str string) (uint32, error) {
	s64, err := strconv.ParseUint(steamID64Str, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid SteamID64 string %s: %w", steamID64Str, err)
	}
	aid := SteamID64ToAccountID(s64)
	if aid == 0 {
		return 0, fmt.Errorf("SteamID64 %d is below the base offset", s64)
	}
	return aid, nil
}

// Backward compatibility aliases matching GLOSSARY.md domain language
var (
	AccountIDToSteam64       = AccountIDToSteamID64
	Steam64ToAccountID       = SteamID64ToAccountID
	AccountIDStringToSteam64 = AccountIDStringToSteamID64
	Steam64StringToAccountID = SteamID64StringToAccountID
)
