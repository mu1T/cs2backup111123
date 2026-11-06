package steam

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"cs2backup/internal/atomicfile"
)

var (
	steamShutdowner = func(steamPath string) error {
		return shutdownSteamWithInterval(steamPath, 10*time.Second, 200*time.Millisecond)
	}
	registrySetter = setSteamAutoLoginRegistry

	// ErrRememberPasswordDisabled is returned when attempting to switch to an account
	// that does not have RememberPassword enabled in loginusers.vdf.
	ErrRememberPasswordDisabled = errors.New("remember password is not enabled for this account")
)

// SetRegistrySetterForTesting allows unit tests to mock registry operations.
func SetRegistrySetterForTesting(fn func(accountName string) error) func() {
	orig := registrySetter
	registrySetter = fn
	return func() {
		registrySetter = orig
	}
}

// SetSteamShutdownerForTesting allows unit tests to mock shutdown behavior.
func SetSteamShutdownerForTesting(fn func(steamPath string) error) func() {
	orig := steamShutdowner
	steamShutdowner = fn
	return func() {
		steamShutdowner = orig
	}
}

type userBlockOffsets struct {
	steam64            string
	accountName        string
	accountIDStr       string
	openBraceOffset    int
	mostRecentValToken *offsetToken
}

type fileEdit struct {
	start       int
	end         int
	replacement []byte
}

func scanUserBlocks(data []byte) ([]userBlockOffsets, error) {
	tokens := tokenizeWithOffsets(data)
	var blocks []userBlockOffsets
	n := len(tokens)

	inUsersSection := false
	i := 0
	for i < n {
		tok := tokens[i]
		if tok.typ == tokenString && strings.EqualFold(tok.val, "users") && i+1 < n && tokens[i+1].typ == tokenOpenBrace {
			inUsersSection = true
			i += 2
			continue
		}

		if inUsersSection && tok.typ == tokenCloseBrace {
			inUsersSection = false
			i++
			continue
		}

		if tok.typ == tokenString && i+1 < n && tokens[i+1].typ == tokenOpenBrace {
			s64Val := tok.val
			s64, err := strconv.ParseUint(s64Val, 10, 64)
			if err == nil && s64 >= SteamUniverseBase {
				aid := Steam64ToAccountID(s64)
				aidStr := strconv.FormatUint(uint64(aid), 10)
				openBraceEnd := tokens[i+1].end

				block := userBlockOffsets{
					steam64:         s64Val,
					accountIDStr:    aidStr,
					openBraceOffset: openBraceEnd,
				}

				depth := 1
				i += 2
				for i < n && depth > 0 {
					subTok := tokens[i]
					if subTok.typ == tokenOpenBrace {
						depth++
						i++
						continue
					}
					if subTok.typ == tokenCloseBrace {
						depth--
						i++
						continue
					}

					if depth == 1 && subTok.typ == tokenString && i+1 < n && tokens[i+1].typ == tokenString {
						key := subTok.val
						valTok := tokens[i+1]
						if strings.EqualFold(key, "accountname") {
							block.accountName = valTok.val
						} else if strings.EqualFold(key, "mostrecent") {
							valCopy := valTok
							block.mostRecentValToken = &valCopy
						}
						i += 2
						continue
					}
					i++
				}
				blocks = append(blocks, block)
				continue
			}
		}
		i++
	}

	return blocks, nil
}

func matchesAccount(block userBlockOffsets, target string) bool {
	if strings.EqualFold(block.accountName, target) {
		return true
	}
	if block.steam64 == target {
		return true
	}
	if block.accountIDStr == target {
		return true
	}
	return false
}

func resolveLoginAccountName(vdfPath string, target string) (string, error) {
	data, err := os.ReadFile(vdfPath)
	if err != nil {
		return "", err
	}
	blocks, err := scanUserBlocks(data)
	if err != nil {
		return "", err
	}
	for _, b := range blocks {
		if matchesAccount(b, target) && b.accountName != "" {
			return b.accountName, nil
		}
	}
	return target, nil
}

// UpdateLoginUsersMostRecent updates loginusers.vdf setting target account's
// MostRecent to "1" and all other accounts to "0", preserving existing comments and structure.
func UpdateLoginUsersMostRecent(vdfPath, targetAccount string) error {
	data, err := os.ReadFile(vdfPath)
	if err != nil {
		return fmt.Errorf("failed to read loginusers.vdf: %w", err)
	}

	blocks, err := scanUserBlocks(data)
	if err != nil {
		return fmt.Errorf("failed to parse loginusers.vdf: %w", err)
	}

	targetIndex := -1
	for idx, b := range blocks {
		if matchesAccount(b, targetAccount) {
			targetIndex = idx
			break
		}
	}

	if targetIndex == -1 {
		return fmt.Errorf("target account %q not found in %s", targetAccount, vdfPath)
	}

	var edits []fileEdit
	for idx, b := range blocks {
		desiredVal := "0"
		if idx == targetIndex {
			desiredVal = "1"
		}

		if b.mostRecentValToken != nil {
			edits = append(edits, fileEdit{
				start:       b.mostRecentValToken.start,
				end:         b.mostRecentValToken.end,
				replacement: []byte("\"" + desiredVal + "\""),
			})
		} else {
			insertStr := "\n\t\t\"MostRecent\"\t\t\"" + desiredVal + "\""
			edits = append(edits, fileEdit{
				start:       b.openBraceOffset,
				end:         b.openBraceOffset,
				replacement: []byte(insertStr),
			})
		}
	}

	sort.Slice(edits, func(i, j int) bool {
		return edits[i].start > edits[j].start
	})

	for _, e := range edits {
		newBytes := make([]byte, 0, len(data)-(e.end-e.start)+len(e.replacement))
		newBytes = append(newBytes, data[:e.start]...)
		newBytes = append(newBytes, e.replacement...)
		newBytes = append(newBytes, data[e.end:]...)
		data = newBytes
	}

	_, err = atomicfile.WriteInPlace(vdfPath, data, atomicfile.Options{})
	return err
}

// SwitchSteamAccount switches the active Steam account:
// 1. Verifies RememberPassword == "1" (true) in loginusers.vdf for the target account.
// 2. Checks if Steam is running. If running, executes graceful shutdown (up to 10s wait).
// 3. Updates Windows registry (AutoLoginUser = accountName, RememberPassword = 1).
// 4. Updates loginusers.vdf (MostRecent = "1" for target, "0" for all other accounts).
// 5. Starts Steam in background mode.
func SwitchSteamAccount(steamPath, accountName string) error {
	if strings.TrimSpace(accountName) == "" {
		return errors.New("accountName cannot be empty")
	}

	if steamPath == "" {
		detected, err := DetectSteamPath()
		if err != nil {
			return fmt.Errorf("steam path not found: %w", err)
		}
		steamPath = detected
	}

	vdfCandidates := []string{
		filepath.Join(steamPath, "config", "loginusers.vdf"),
		filepath.Join(steamPath, "userdata", "0", "config", "loginusers.vdf"),
	}
	var loginVDFPath string
	for _, cand := range vdfCandidates {
		if _, err := os.Stat(cand); err == nil {
			loginVDFPath = cand
			break
		}
	}

	targetLoginName := accountName
	if loginVDFPath != "" {
		users, err := ParseLoginUsersFile(loginVDFPath)
		if err != nil {
			return fmt.Errorf("failed to parse loginusers.vdf: %w", err)
		}

		var matchedUser *LoginUserInfo
		for _, u := range users {
			if strings.EqualFold(u.AccountName, accountName) ||
				strconv.FormatUint(u.SteamID64, 10) == accountName ||
				strconv.FormatUint(uint64(u.AccountID), 10) == accountName {
				matchedUser = u
				break
			}
		}

		if matchedUser == nil {
			return fmt.Errorf("target account %q not found in %s", accountName, loginVDFPath)
		}

		if !matchedUser.RememberPassword {
			return fmt.Errorf("account %s does not have RememberPassword enabled: %w", accountName, ErrRememberPasswordDisabled)
		}

		if matchedUser.AccountName != "" {
			targetLoginName = matchedUser.AccountName
		}
	}

	// 1. Process running check and graceful shutdown
	running, err := IsSteamRunning()
	if err != nil {
		return fmt.Errorf("failed to check steam running state: %w", err)
	}
	if running {
		if err := steamShutdowner(steamPath); err != nil {
			return fmt.Errorf("failed to shutdown steam: %w", err)
		}
	}

	// 2. Set Windows registry
	if err := registrySetter(targetLoginName); err != nil {
		return fmt.Errorf("failed to set steam registry autologin: %w", err)
	}

	// 3. Update loginusers.vdf
	if loginVDFPath != "" {
		if err := UpdateLoginUsersMostRecent(loginVDFPath, accountName); err != nil {
			return fmt.Errorf("failed to update loginusers.vdf: %w", err)
		}
	}

	// 4. Start Steam
	if err := StartSteam(steamPath); err != nil {
		return fmt.Errorf("failed to start steam: %w", err)
	}

	return nil
}
