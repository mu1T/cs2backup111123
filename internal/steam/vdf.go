package steam

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// LoginUserInfo stores user metadata extracted from loginusers.vdf.
type LoginUserInfo struct {
	SteamID64        uint64 `json:"steam64"`
	AccountID        uint32 `json:"account_id"`
	AccountName      string `json:"account_name"`
	PersonaName      string `json:"persona_name"`
	RememberPassword bool   `json:"remember_password"`
	MostRecent       bool   `json:"most_recent"`
	Timestamp        int64  `json:"timestamp"`
}

type tokenType int

const (
	tokenEOF tokenType = iota
	tokenString
	tokenOpenBrace
	tokenCloseBrace
)

type token struct {
	typ tokenType
	val string
}

var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// vdfTokenizer scans VDF streams without regular expressions or large buffers.
type vdfTokenizer struct {
	reader *bufio.Reader
}

// ScanVDFTokens tokenizes all tokens from an io.Reader and returns the token count.
func ScanVDFTokens(r io.Reader) (int, error) {
	tok := newVDFTokenizer(r)
	count := 0
	for {
		t, err := tok.nextToken()
		if err != nil {
			return count, err
		}
		if t.typ == tokenEOF {
			break
		}
		count++
	}
	return count, nil
}

func newVDFTokenizer(r io.Reader) *vdfTokenizer {
	return &vdfTokenizer{
		reader: bufio.NewReaderSize(r, 8192),
	}
}

func (t *vdfTokenizer) nextToken() (token, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	for {
		b, err := t.reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return token{typ: tokenEOF}, nil
			}
			return token{}, err
		}

		// Skip whitespace
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			continue
		}

		// Comment '//' -> skip to line end
		if b == '/' {
			peek, err := t.reader.Peek(1)
			if err == nil && len(peek) > 0 && peek[0] == '/' {
				// Consume until \n or EOF
				_, _ = t.reader.ReadByte() // consume second '/'
				for {
					cb, err := t.reader.ReadByte()
					if err != nil || cb == '\n' {
						break
					}
				}
				continue
			}
		}

		// Structural braces
		if b == '{' {
			return token{typ: tokenOpenBrace, val: "{"}, nil
		}
		if b == '}' {
			return token{typ: tokenCloseBrace, val: "}"}, nil
		}

		// Quoted string
		if b == '"' {
			for {
				cb, err := t.reader.ReadByte()
				if err != nil {
					return token{typ: tokenString, val: buf.String()}, nil
				}
				if cb == '\\' {
					escaped, err := t.reader.ReadByte()
					if err != nil {
						buf.WriteByte(cb)
						break
					}
					switch escaped {
					case '"':
						buf.WriteByte('"')
					case '\\':
						buf.WriteByte('\\')
					case 'n':
						buf.WriteByte('\n')
					case 't':
						buf.WriteByte('\t')
					default:
						buf.WriteByte('\\')
						buf.WriteByte(escaped)
					}
					continue
				}
				if cb == '"' {
					return token{typ: tokenString, val: buf.String()}, nil
				}
				buf.WriteByte(cb)
			}
		}

		// Unquoted token
		buf.WriteByte(b)
		for {
			peek, err := t.reader.Peek(1)
			if err != nil {
				break
			}
			pb := peek[0]
			if pb == ' ' || pb == '\t' || pb == '\r' || pb == '\n' || pb == '{' || pb == '}' || pb == '"' {
				break
			}
			_, _ = t.reader.ReadByte()
			buf.WriteByte(pb)
		}
		return token{typ: tokenString, val: buf.String()}, nil
	}
}

// ParseLoginUsers parses a stream of loginusers.vdf and returns a map of AccountID -> LoginUserInfo.
// It also ensures that the most recent account is properly flagged (MostRecent = true).
func ParseLoginUsers(r io.Reader) (map[uint32]*LoginUserInfo, error) {
	tok := newVDFTokenizer(r)

	// Structure:
	// "users" {
	//    "76561198083722517" {
	//         "AccountName" "..."
	//         "PersonaName" "..."
	//         "MostRecent"  "1"
	//         "Timestamp"   "..."
	//    }
	// }
	users := make(map[uint32]*LoginUserInfo)

	for {
		t, err := tok.nextToken()
		if err != nil {
			return nil, err
		}
		if t.typ == tokenEOF {
			break
		}

		if t.typ == tokenString {
			key := t.val
			// Next token might be '{' or another string (key-value)
			next, err := tok.nextToken()
			if err != nil {
				return nil, err
			}

			if next.typ == tokenOpenBrace {
				// Section entered. If key is "users", parse inner user sections.
				// If key is a numeric steam64, parse it directly as a user section.
				if strings.EqualFold(key, "users") {
					if err := parseUsersSection(tok, users); err != nil {
						return nil, err
					}
				} else if s64, err := strconv.ParseUint(key, 10, 64); err == nil && s64 >= SteamUniverseBase {
					u := parseUserBlock(tok, s64)
					if u != nil {
						users[u.AccountID] = u
					}
				} else {
					// Skip generic section
					skipSection(tok)
				}
			}
		}
	}

	// Verify if any user has MostRecent == true
	var activeUser *LoginUserInfo
	var newestUser *LoginUserInfo
	for _, u := range users {
		if u.MostRecent {
			if activeUser == nil || u.Timestamp > activeUser.Timestamp {
				activeUser = u
			}
		}
		if newestUser == nil || u.Timestamp > newestUser.Timestamp {
			newestUser = u
		}
	}

	// If no user has MostRecent == true, fall back to the one with the latest timestamp
	if activeUser == nil && newestUser != nil {
		newestUser.MostRecent = true
		activeUser = newestUser
	}

	// Ensure only ONE user has MostRecent == true
	if activeUser != nil {
		for _, u := range users {
			u.MostRecent = (u.AccountID == activeUser.AccountID)
		}
	}

	return users, nil
}

// ParseLoginUsersFile parses the loginusers.vdf file at the specified path.
func ParseLoginUsersFile(filePath string) (map[uint32]*LoginUserInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open loginusers.vdf: %w", err)
	}
	defer f.Close()

	return ParseLoginUsers(f)
}

func parseUsersSection(tok *vdfTokenizer, users map[uint32]*LoginUserInfo) error {
	for {
		t, err := tok.nextToken()
		if err != nil {
			return err
		}
		if t.typ == tokenEOF || t.typ == tokenCloseBrace {
			return nil
		}

		if t.typ == tokenString {
			key := t.val
			next, err := tok.nextToken()
			if err != nil {
				return err
			}
			if next.typ == tokenOpenBrace {
				if s64, err := strconv.ParseUint(key, 10, 64); err == nil && s64 >= SteamUniverseBase {
					u := parseUserBlock(tok, s64)
					if u != nil {
						users[u.AccountID] = u
					}
				} else {
					skipSection(tok)
				}
			}
		}
	}
}

func parseUserBlock(tok *vdfTokenizer, steam64 uint64) *LoginUserInfo {
	u := &LoginUserInfo{
		SteamID64: steam64,
		AccountID: Steam64ToAccountID(steam64),
	}

	for {
		t, err := tok.nextToken()
		if err != nil || t.typ == tokenEOF || t.typ == tokenCloseBrace {
			break
		}

		if t.typ == tokenString {
			key := t.val
			next, err := tok.nextToken()
			if err != nil || next.typ == tokenEOF {
				break
			}

			if next.typ == tokenOpenBrace {
				// Nested section inside user, skip it
				skipSection(tok)
				continue
			}

			if next.typ == tokenString {
				val := next.val
				switch strings.ToLower(key) {
				case "personaname":
					u.PersonaName = val
				case "accountname":
					u.AccountName = val
				case "rememberpassword":
					u.RememberPassword = (val == "1" || strings.EqualFold(val, "true"))
				case "mostrecent":
					u.MostRecent = (val == "1" || strings.EqualFold(val, "true"))
				case "timestamp":
					if ts, err := strconv.ParseInt(val, 10, 64); err == nil {
						u.Timestamp = ts
					}
				}
			}
		}
	}

	return u
}

func skipSection(tok *vdfTokenizer) {
	depth := 1
	for depth > 0 {
		t, err := tok.nextToken()
		if err != nil || t.typ == tokenEOF {
			break
		}
		if t.typ == tokenOpenBrace {
			depth++
		} else if t.typ == tokenCloseBrace {
			depth--
		}
	}
}

// LibraryFolder represents a Steam library folder entry.
type LibraryFolder struct {
	Path string           `json:"path"`
	Apps map[string]int64 `json:"apps"`
}

// ParseLibraryFolders parses libraryfolders.vdf from an io.Reader.
func ParseLibraryFolders(r io.Reader) ([]LibraryFolder, error) {
	tok := newVDFTokenizer(r)
	var folders []LibraryFolder

	for {
		t, err := tok.nextToken()
		if err != nil {
			return nil, err
		}
		if t.typ == tokenEOF {
			break
		}

		if t.typ == tokenString {
			key := t.val
			next, err := tok.nextToken()
			if err != nil {
				return nil, err
			}

			if next.typ == tokenOpenBrace {
				if strings.EqualFold(key, "libraryfolders") {
					for {
						et, err := tok.nextToken()
						if err != nil || et.typ == tokenEOF || et.typ == tokenCloseBrace {
							break
						}

						if et.typ == tokenString {
							entryKey := et.val
							entryNext, err := tok.nextToken()
							if err != nil || entryNext.typ == tokenEOF {
								break
							}

							if entryNext.typ == tokenOpenBrace {
								lf := LibraryFolder{Apps: make(map[string]int64)}
								for {
									it, err := tok.nextToken()
									if err != nil || it.typ == tokenEOF || it.typ == tokenCloseBrace {
										break
									}

									if it.typ == tokenString {
										itemKey := it.val
										itemValTok, err := tok.nextToken()
										if err != nil || itemValTok.typ == tokenEOF {
											break
										}

										if itemValTok.typ == tokenOpenBrace {
											if strings.EqualFold(itemKey, "apps") {
												for {
													at, err := tok.nextToken()
													if err != nil || at.typ == tokenEOF || at.typ == tokenCloseBrace {
														break
													}
													if at.typ == tokenString {
														appID := at.val
														sizeTok, err := tok.nextToken()
														if err != nil || sizeTok.typ == tokenEOF {
															break
														}
														if sizeTok.typ == tokenString {
															size, _ := strconv.ParseInt(sizeTok.val, 10, 64)
															lf.Apps[appID] = size
														}
													}
												}
											} else {
												skipSection(tok)
											}
										} else if itemValTok.typ == tokenString {
											if strings.EqualFold(itemKey, "path") {
												lf.Path = filepath.Clean(itemValTok.val)
											}
										}
									}
								}
								if lf.Path != "" {
									folders = append(folders, lf)
								}
							} else if entryNext.typ == tokenString {
								// Legacy format: "1" "D:\\SteamLibrary"
								if _, err := strconv.Atoi(entryKey); err == nil && entryNext.val != "" {
									folders = append(folders, LibraryFolder{
										Path: filepath.Clean(entryNext.val),
										Apps: make(map[string]int64),
									})
								}
							}
						}
					}
				} else {
					skipSection(tok)
				}
			}
		}
	}

	return folders, nil
}

// ParseLibraryFoldersFile parses the libraryfolders.vdf at the given path.
func ParseLibraryFoldersFile(filePath string) ([]LibraryFolder, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open libraryfolders.vdf: %w", err)
	}
	defer f.Close()

	return ParseLibraryFolders(f)
}

// ParsePersonaName 从任意 VDF 里取出第一个 PersonaName 的值。
//
// userdata/<id>/config/localconfig.vdf 里就带着 Steam 自己写的昵称，位置在深层嵌套里，
// 但「键值是两个连续字符串」这条规律在任何层级都成立，所以直接扫一遍即可，
// 不必为它写一套完整的结构体。
func ParsePersonaName(r io.Reader) (string, error) {
	tok := newVDFTokenizer(r)
	pending := false
	for {
		t, err := tok.nextToken()
		if err != nil {
			return "", err
		}
		// 分词器在文件结束时返回 tokenEOF 而**不是** error，必须显式判掉，
		// 否则这里会空转（踩过一次：测试直接卡死被 kill）。
		if t.typ == tokenEOF {
			return "", nil
		}
		if t.typ != tokenString {
			pending = false
			continue
		}
		if pending {
			return strings.TrimSpace(t.val), nil
		}
		pending = t.val == "PersonaName"
	}
}

// ReadLocalPersonaName 读取 userdata/<accountID>/config/localconfig.vdf 里的昵称。
//
// 这是离线可用的本地来源：比走 Steam 社区 XML 更快，也不受隐私设置 / 网络影响。
// 文件不存在或没有该键时返回空串，不算错误。
func ReadLocalPersonaName(steamPath, accountID string) string {
	if steamPath == "" || accountID == "" {
		return ""
	}
	path := filepath.Join(steamPath, "userdata", accountID, "config", "localconfig.vdf")
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	name, err := ParsePersonaName(bufio.NewReader(f))
	if err != nil {
		return ""
	}
	return name
}
