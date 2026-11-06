package steam

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cs2backup/internal/atomicfile"
)

// SanitizeLaunchOptions cleans launch options string by removing control characters
// and normalizing multiple spaces, tabs, and newlines.
func SanitizeLaunchOptions(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	space := false
	for _, r := range s {
		if r == '\r' || r == '\n' || r == '\t' || r == ' ' {
			if !space && buf.Len() > 0 {
				buf.WriteByte(' ')
				space = true
			}
		} else {
			buf.WriteRune(r)
			space = false
		}
	}
	return strings.TrimSpace(buf.String())
}

// EscapeVDFString escapes backslashes and double quotes for VDF serialization.
func EscapeVDFString(s string) string {
	var buf strings.Builder
	buf.Grow(len(s) + 16)
	for i := 0; i < len(s); i++ {
		b := s[i]
		switch b {
		case '\\':
			buf.WriteString(`\\`)
		case '"':
			buf.WriteString(`\"`)
		default:
			buf.WriteByte(b)
		}
	}
	return buf.String()
}

// UnescapeVDFString unescapes VDF escaped characters (\", \\, \n, \t).
func UnescapeVDFString(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == '\\' && i+1 < len(s) {
			escaped := s[i+1]
			switch escaped {
			case '"':
				buf.WriteByte('"')
				i++
			case '\\':
				buf.WriteByte('\\')
				i++
			case 'n':
				buf.WriteByte('\n')
				i++
			case 't':
				buf.WriteByte('\t')
				i++
			default:
				buf.WriteByte(b)
			}
			continue
		}
		buf.WriteByte(b)
	}
	return buf.String()
}

// ParseLaunchOptions reads localconfig.vdf from an io.Reader and locates CS2 AppID 730 LaunchOptions.
func ParseLaunchOptions(r io.Reader) (string, error) {
	tok := newVDFTokenizer(r)
	var stack []string

	for {
		t, err := tok.nextToken()
		if err != nil {
			return "", err
		}
		if t.typ == tokenEOF {
			break
		}

		if t.typ == tokenCloseBrace {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}

		if t.typ == tokenString {
			key := t.val
			next, err := tok.nextToken()
			if err != nil {
				return "", err
			}
			if next.typ == tokenEOF {
				break
			}

			if next.typ == tokenOpenBrace {
				stack = append(stack, strings.ToLower(key))
				continue
			}

			if next.typ == tokenString {
				if strings.EqualFold(key, "LaunchOptions") && isInsideApp730(stack) {
					return next.val, nil
				}
			}
		}
	}

	return "", nil
}

func isInsideApp730(stack []string) bool {
	if len(stack) == 0 {
		return false
	}
	if stack[len(stack)-1] != "730" {
		return false
	}
	for i := 0; i < len(stack)-1; i++ {
		if stack[i] == "apps" {
			return true
		}
	}
	return false
}

// LaunchOptionsManager provides access to reading and writing CS2 launch options for Steam accounts.
type LaunchOptionsManager struct {
	steamPath string
}

// NewLaunchOptionsManager creates a new LaunchOptionsManager with the given Steam installation root.
func NewLaunchOptionsManager(steamPath string) *LaunchOptionsManager {
	return &LaunchOptionsManager{
		steamPath: steamPath,
	}
}

// GetLocalConfigPath returns the absolute path to localconfig.vdf for a specified account.
func (m *LaunchOptionsManager) GetLocalConfigPath(accountID string) string {
	return filepath.Join(m.steamPath, "userdata", accountID, "config", "localconfig.vdf")
}

// GetLaunchOptions retrieves CS2 launch options from userdata/<accountID>/config/localconfig.vdf.
func (m *LaunchOptionsManager) GetLaunchOptions(accountID string) (string, error) {
	if m.steamPath == "" {
		return "", ErrSteamNotFound
	}

	filePath := m.GetLocalConfigPath(accountID)
	f, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("failed to open localconfig.vdf: %w", err)
	}
	defer f.Close()

	return ParseLaunchOptions(f)
}

type offsetToken struct {
	typ   tokenType
	val   string
	start int
	end   int
}

// tokenizeWithOffsets scans a VDF byte slice and emits tokens with their byte slice start and end offsets.
func tokenizeWithOffsets(data []byte) []offsetToken {
	var tokens []offsetToken
	n := len(data)
	i := 0

	for i < n {
		b := data[i]

		// Skip whitespace
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			i++
			continue
		}

		// Comment
		if b == '/' && i+1 < n && data[i+1] == '/' {
			i += 2
			for i < n && data[i] != '\n' {
				i++
			}
			continue
		}

		// Braces
		if b == '{' {
			tokens = append(tokens, offsetToken{typ: tokenOpenBrace, val: "{", start: i, end: i + 1})
			i++
			continue
		}
		if b == '}' {
			tokens = append(tokens, offsetToken{typ: tokenCloseBrace, val: "}", start: i, end: i + 1})
			i++
			continue
		}

		// Quoted string
		if b == '"' {
			start := i
			i++ // skip opening quote
			var valBuf strings.Builder
			for i < n {
				cb := data[i]
				if cb == '\\' && i+1 < n {
					escaped := data[i+1]
					switch escaped {
					case '"':
						valBuf.WriteByte('"')
					case '\\':
						valBuf.WriteByte('\\')
					case 'n':
						valBuf.WriteByte('\n')
					case 't':
						valBuf.WriteByte('\t')
					default:
						valBuf.WriteByte('\\')
						valBuf.WriteByte(escaped)
					}
					i += 2
					continue
				}
				if cb == '"' {
					i++ // skip closing quote
					break
				}
				valBuf.WriteByte(cb)
				i++
			}
			tokens = append(tokens, offsetToken{
				typ:   tokenString,
				val:   valBuf.String(),
				start: start,
				end:   i,
			})
			continue
		}

		// Unquoted token
		start := i
		for i < n {
			pb := data[i]
			if pb == ' ' || pb == '\t' || pb == '\r' || pb == '\n' || pb == '{' || pb == '}' || pb == '"' {
				break
			}
			i++
		}
		tokens = append(tokens, offsetToken{
			typ:   tokenString,
			val:   string(data[start:i]),
			start: start,
			end:   i,
		})
	}

	return tokens
}

// SetLaunchOptions updates the LaunchOptions key under AppID 730 in localconfig.vdf,
// preserving exact existing formatting, indentation, and structure without regex backtracking.
func (m *LaunchOptionsManager) SetLaunchOptions(accountID, options string) error {
	if m.steamPath == "" {
		return ErrSteamNotFound
	}

	filePath := m.GetLocalConfigPath(accountID)
	cfgDir := filepath.Dir(filePath)
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	sanitized := SanitizeLaunchOptions(options)
	escapedVal := EscapeVDFString(sanitized)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Create new skeleton
			skeleton := fmt.Sprintf("\"UserLocalConfigStore\"\n{\n\t\"Software\"\n\t{\n\t\t\"Valve\"\n\t\t{\n\t\t\t\"Steam\"\n\t\t\t{\n\t\t\t\t\"Apps\"\n\t\t\t\t{\n\t\t\t\t\t\"730\"\n\t\t\t\t\t{\n\t\t\t\t\t\t\"LaunchOptions\"\t\t\"%s\"\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n}\n", escapedVal)
			_, err := atomicfile.WriteInPlace(filePath, []byte(skeleton), atomicfile.Options{})
			return err
		}
		return fmt.Errorf("failed to read localconfig.vdf: %w", err)
	}

	// Detect newline style
	nl := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		nl = "\r\n"
	}

	tokens := tokenizeWithOffsets(data)

	var stack []string
	var launchOptionsValToken *offsetToken
	cs2SectionOpen := -1
	appsSectionOpen := -1
	steamSectionOpen := -1

	for idx := 0; idx < len(tokens); idx++ {
		tok := tokens[idx]

		if tok.typ == tokenCloseBrace {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}

		if tok.typ == tokenString {
			key := tok.val
			if idx+1 >= len(tokens) {
				break
			}
			next := tokens[idx+1]
			idx++ // consume next

			if next.typ == tokenOpenBrace {
				stack = append(stack, strings.ToLower(key))
				if strings.EqualFold(key, "Steam") {
					steamSectionOpen = next.end
				} else if strings.EqualFold(key, "Apps") {
					appsSectionOpen = next.end
				} else if strings.EqualFold(key, "730") && isInsideApps(stack) {
					cs2SectionOpen = next.end
				}
				continue
			}

			if next.typ == tokenString {
				if strings.EqualFold(key, "LaunchOptions") && isInsideApp730(stack) {
					tokenCopy := next
					launchOptionsValToken = &tokenCopy
					break
				}
			}
		}
	}

	var newBytes []byte

	if launchOptionsValToken != nil {
		// Case 1: LaunchOptions exists in 730 -> replace value string directly preserving surroundings
		newBytes = make([]byte, 0, len(data)+len(escapedVal)+2)
		newBytes = append(newBytes, data[:launchOptionsValToken.start]...)
		newBytes = append(newBytes, []byte("\""+escapedVal+"\"")...)
		newBytes = append(newBytes, data[launchOptionsValToken.end:]...)
	} else if cs2SectionOpen != -1 {
		// Case 2: 730 exists, but LaunchOptions does not -> insert right after 730's {
		insertStr := nl + "\t\t\t\t\t\t\"LaunchOptions\"\t\t\"" + escapedVal + "\""
		newBytes = make([]byte, 0, len(data)+len(insertStr))
		newBytes = append(newBytes, data[:cs2SectionOpen]...)
		newBytes = append(newBytes, []byte(insertStr)...)
		newBytes = append(newBytes, data[cs2SectionOpen:]...)
	} else if appsSectionOpen != -1 {
		// Case 3: Apps exists, but 730 does not -> insert 730 block right after Apps's {
		insertStr := nl + "\t\t\t\t\t\"730\"" + nl + "\t\t\t\t\t{" + nl + "\t\t\t\t\t\t\"LaunchOptions\"\t\t\"" + escapedVal + "\"" + nl + "\t\t\t\t\t}"
		newBytes = make([]byte, 0, len(data)+len(insertStr))
		newBytes = append(newBytes, data[:appsSectionOpen]...)
		newBytes = append(newBytes, []byte(insertStr)...)
		newBytes = append(newBytes, data[appsSectionOpen:]...)
	} else if steamSectionOpen != -1 {
		// Case 4: Steam exists, but Apps does not -> insert Apps and 730
		insertStr := nl + "\t\t\t\t\"Apps\"" + nl + "\t\t\t\t{" + nl + "\t\t\t\t\t\"730\"" + nl + "\t\t\t\t\t{" + nl + "\t\t\t\t\t\t\"LaunchOptions\"\t\t\"" + escapedVal + "\"" + nl + "\t\t\t\t\t}" + nl + "\t\t\t\t}"
		newBytes = make([]byte, 0, len(data)+len(insertStr))
		newBytes = append(newBytes, data[:steamSectionOpen]...)
		newBytes = append(newBytes, []byte(insertStr)...)
		newBytes = append(newBytes, data[steamSectionOpen:]...)
	} else {
		// Fallback skeleton
		skeleton := fmt.Sprintf("\"UserLocalConfigStore\"\n{\n\t\"Software\"\n\t{\n\t\t\"Valve\"\n\t\t{\n\t\t\t\"Steam\"\n\t\t\t{\n\t\t\t\t\"Apps\"\n\t\t\t\t{\n\t\t\t\t\t\"730\"\n\t\t\t\t\t{\n\t\t\t\t\t\t\"LaunchOptions\"\t\t\"%s\"\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n}\n", escapedVal)
		newBytes = []byte(skeleton)
	}

	_, err = atomicfile.WriteInPlace(filePath, newBytes, atomicfile.Options{})
	return err
}

func isInsideApps(stack []string) bool {
	for _, s := range stack {
		if s == "apps" {
			return true
		}
	}
	return false
}

