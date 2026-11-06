package lan

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PackageMetadata contains descriptive information and file hashes stored in metadata.json within a LAN Package.
const copyBufferSize = 64 * 1024 // 64KB per ADR 0002

var copyBufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, copyBufferSize)
		return &buf
	},
}

type PackageMetadata struct {
	Name             string            `json:"name"`
	AccountID        string            `json:"account_id,omitempty"`
	Created          string            `json:"created"`
	ContainsCfg      bool              `json:"contains_cfg"`
	ContainsUserdata bool              `json:"contains_userdata"`
	SteamUserdata    string            `json:"steam_userdata_path,omitempty"`
	CS2CfgPath       string            `json:"cs2_cfg_path,omitempty"`
	Files            []PackageFileInfo `json:"files"`
	Type             string            `json:"type,omitempty"`
}

// PackageFileInfo holds metadata for an individual file inside a LAN Package.
type PackageFileInfo struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// PackageInfo provides summary information about a created LAN Package on disk.
type PackageInfo struct {
	Filename string `json:"filename"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
}

// PackageOptions configures creation of a LAN Package.
type PackageOptions struct {
	AccountID       string
	Name            string
	IncludeUserdata bool
	UserdataDir     string // Root of 730/ folder or directory containing 730/
	IncludeCfg      bool
	CfgDir          string // Root of cfg files
	OutputPath      string
}

// PackageValidationResult reports the validation outcome of a LAN Package.
type PackageValidationResult struct {
	Valid         bool                 `json:"valid"`
	AllValid      bool                 `json:"all_valid"`
	HasRestoreBat bool                 `json:"has_restore_bat"`
	Metadata      PackageMetadata      `json:"metadata"`
	Validation    []FileValidationItem `json:"validation"`
}

// FileValidationItem reports the check status of an individual file in a package.
type FileValidationItem struct {
	Path     string `json:"path"`
	Valid    bool   `json:"valid"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Error    string `json:"error,omitempty"`
}

// GenerateRestoreBat creates the Windows restore.bat script content.
func GenerateRestoreBat(hasCfg, hasUserdata bool, targetAccountID ...string) string {
	var sb strings.Builder
	sb.WriteString("@echo off\r\n")
	sb.WriteString("chcp 65001 >nul\r\n")
	sb.WriteString("setlocal enabledelayedexpansion\r\n")
	sb.WriteString("title CS2 CFG Backup Restore\r\n\r\n")
	sb.WriteString("echo.\r\n")
	sb.WriteString("echo  ============================================\r\n")
	sb.WriteString("echo         CS2 CFG Backup Restore\r\n")
	sb.WriteString("echo  ============================================\r\n")
	sb.WriteString("echo.\r\n\r\n")

	sb.WriteString(":: ===== Detect Steam =====\r\n")
	sb.WriteString("set \"STEAM_PATH=\"\r\n\r\n")
	sb.WriteString("for %%D in (C D E F G H) do (\r\n")
	sb.WriteString("    for %%P in (\r\n")
	sb.WriteString("        \"%%D:\\Steam\"\r\n")
	sb.WriteString("        \"%%D:\\Program Files (x86)\\Steam\"\r\n")
	sb.WriteString("        \"%%D:\\Program Files\\Steam\"\r\n")
	sb.WriteString("    ) do (\r\n")
	sb.WriteString("        if exist \"%%~P\\steam.exe\" (\r\n")
	sb.WriteString("            set \"STEAM_PATH=%%~P\"\r\n")
	sb.WriteString("            echo  [OK] Steam: %%~P\r\n")
	sb.WriteString("        )\r\n")
	sb.WriteString("    )\r\n")
	sb.WriteString(")\r\n\r\n")

	sb.WriteString("if \"!STEAM_PATH!\"==\"\" (\r\n")
	sb.WriteString("    echo.\r\n")
	sb.WriteString("    echo  [ERROR] Steam not found!\r\n")
	sb.WriteString("    echo  Please install Steam or place this BAT next to Steam.\r\n")
	sb.WriteString("    echo.\r\n")
	sb.WriteString("    goto :DONE\r\n")
	sb.WriteString(")\r\n\r\n")

	if hasCfg {
		sb.WriteString("echo.\r\n")
		sb.WriteString("echo ===== CFG Restore =====\r\n\r\n")
		sb.WriteString("set \"CS2_CFG=\"\r\n\r\n")
		sb.WriteString("if exist \"!STEAM_PATH!\\steamapps\\common\\Counter-Strike Global Offensive\\game\\csgo\\cfg\" (\r\n")
		sb.WriteString("    set \"CS2_CFG=!STEAM_PATH!\\steamapps\\common\\Counter-Strike Global Offensive\\game\\csgo\\cfg\"\r\n")
		sb.WriteString(")\r\n\r\n")

		sb.WriteString("if \"!CS2_CFG!\"==\"\" (\r\n")
		sb.WriteString("    if exist \"!STEAM_PATH!\\steamapps\\libraryfolders.vdf\" (\r\n")
		sb.WriteString("        for /f \"tokens=2 delims= \" %%a in ('findstr /r \"path\" \"!STEAM_PATH!\\steamapps\\libraryfolders.vdf\"') do (\r\n")
		sb.WriteString("            if exist \"%%~a\\steamapps\\common\\Counter-Strike Global Offensive\\game\\csgo\\cfg\" (\r\n")
		sb.WriteString("                set \"CS2_CFG=%%~a\\steamapps\\common\\Counter-Strike Global Offensive\\game\\csgo\\cfg\"\r\n")
		sb.WriteString("            )\r\n")
		sb.WriteString("        )\r\n")
		sb.WriteString("    )\r\n")
		sb.WriteString(")\r\n\r\n")

		sb.WriteString("if \"!CS2_CFG!\"==\"\" (\r\n")
		sb.WriteString("    for %%D in (C D E F G H) do (\r\n")
		sb.WriteString("        for %%L in (\"%%D:\\SteamLibrary\" \"%%D:\\Steam\") do (\r\n")
		sb.WriteString("            if exist \"%%~L\\steamapps\\common\\Counter-Strike Global Offensive\\game\\csgo\\cfg\" (\r\n")
		sb.WriteString("                set \"CS2_CFG=%%~L\\steamapps\\common\\Counter-Strike Global Offensive\\game\\csgo\\cfg\"\r\n")
		sb.WriteString("            )\r\n")
		sb.WriteString("        )\r\n")
		sb.WriteString("    )\r\n")
		sb.WriteString(")\r\n\r\n")

		sb.WriteString("if \"!CS2_CFG!\"==\"\" (\r\n")
		sb.WriteString("    echo  [WARN] CS2 cfg directory not found.\r\n")
		sb.WriteString("    echo  Opening backup cfg folder for manual copy...\r\n")
		sb.WriteString("    start \"\" \"%~dp0cfg\"\r\n")
		sb.WriteString(") else (\r\n")
		sb.WriteString("    echo  [OK] CS2 cfg: !CS2_CFG!\r\n")
		sb.WriteString("    echo.\r\n")
		sb.WriteString("    echo  Copying CFG files with xcopy/robocopy...\r\n")
		sb.WriteString("    if exist \"%~dp0cfg\" (\r\n")
		sb.WriteString("        xcopy \"%~dp0cfg\" \"!CS2_CFG!\\\" /e /i /y >nul 2>&1\r\n")
		sb.WriteString("        if !errorlevel! neq 0 (\r\n")
		sb.WriteString("            robocopy \"%~dp0cfg\" \"!CS2_CFG!\" /e /np /nfl /ndl >nul 2>&1\r\n")
		sb.WriteString("        )\r\n")
		sb.WriteString("        echo  [OK] CFG files successfully copied to !CS2_CFG!\r\n")
		sb.WriteString("    )\r\n")
		sb.WriteString(")\r\n\r\n")
	}

	if hasUserdata {
		prefilledAcc := ""
		if len(targetAccountID) > 0 && targetAccountID[0] != "" {
			prefilledAcc = targetAccountID[0]
		}

		sb.WriteString("echo.\r\n")
		sb.WriteString("echo ===== Userdata Restore =====\r\n\r\n")
		sb.WriteString("set \"UD_DIR=\"\r\n")
		sb.WriteString("if exist \"!STEAM_PATH!\\userdata\" set \"UD_DIR=!STEAM_PATH!\\userdata\"\r\n\r\n")
		sb.WriteString("if \"!UD_DIR!\"==\"\" (\r\n")
		sb.WriteString("    echo  [WARN] Steam userdata not found.\r\n")
		sb.WriteString("    goto :SKIP_USERDATA\r\n")
		sb.WriteString(")\r\n\r\n")
		sb.WriteString(fmt.Sprintf("set \"ACCOUNT_ID=%s\"\r\n\r\n", prefilledAcc))
		sb.WriteString("if \"!ACCOUNT_ID!\"==\"\" (\r\n")
		sb.WriteString("    for /d %%U in (\"!UD_DIR!\\*\") do (\r\n")
		sb.WriteString("        if not \"%%~nxU\"==\"0\" if not \"%%~nxU\"==\"anonymous\" (\r\n")
		sb.WriteString("            set \"ACCOUNT_ID=%%~nxU\"\r\n")
		sb.WriteString("        )\r\n")
		sb.WriteString("    )\r\n")
		sb.WriteString(")\r\n\r\n")
		sb.WriteString("if not \"!ACCOUNT_ID!\"==\"\" (\r\n")
		sb.WriteString("    set \"DEST_730=!UD_DIR!\\!ACCOUNT_ID!\\730\"\r\n")
		sb.WriteString("    echo  Target Account ID: !ACCOUNT_ID!\r\n")
		sb.WriteString("    echo  Restoring 730 user configuration to !DEST_730!...\r\n")
		sb.WriteString("    if not exist \"!DEST_730!\" mkdir \"!DEST_730!\"\r\n")
		sb.WriteString("    set \"SRC_730=\"\r\n")
		sb.WriteString("    if exist \"%~dp0730\" set \"SRC_730=%~dp0730\"\r\n")
		sb.WriteString("    if exist \"%~dp0userdata\\730\" set \"SRC_730=%~dp0userdata\\730\"\r\n")
		sb.WriteString("    if \"!SRC_730!\"==\"\" if exist \"%~dp0userdata\" set \"SRC_730=%~dp0userdata\"\r\n")
		sb.WriteString("    if not \"!SRC_730!\"==\"\" (\r\n")
		sb.WriteString("        xcopy \"!SRC_730!\" \"!DEST_730!\\\" /e /i /y >nul 2>&1\r\n")
		sb.WriteString("        if !errorlevel! neq 0 (\r\n")
		sb.WriteString("            robocopy \"!SRC_730!\" \"!DEST_730!\" /e /np /nfl /ndl >nul 2>&1\r\n")
		sb.WriteString("        )\r\n")
		sb.WriteString("        echo  [OK] Userdata 730 files copied to !DEST_730!\r\n")
		sb.WriteString("    )\r\n")
		sb.WriteString(") else (\r\n")
		sb.WriteString("    echo  [WARN] No Account ID specified. Opening folders for manual copy...\r\n")
		sb.WriteString("    if exist \"%~dp0730\" start \"\" \"%~dp0730\"\r\n")
		sb.WriteString("    start \"\" \"!UD_DIR!\"\r\n")
		sb.WriteString(")\r\n\r\n")
		sb.WriteString(":SKIP_USERDATA\r\n")
	}

	sb.WriteString("echo.\r\n")
	sb.WriteString("echo  ============================================\r\n")
	sb.WriteString("echo   Restore complete!\r\n")
	sb.WriteString("echo  ============================================\r\n")
	sb.WriteString("echo.\r\n")
	sb.WriteString("echo  You can close this window and launch CS2.\r\n")
	sb.WriteString("echo.\r\n\r\n")
	sb.WriteString(":DONE\r\n")
	sb.WriteString("echo  Press any key to exit...\r\n")
	sb.WriteString("pause >nul\r\n")

	return sb.String()
}

func SanitizeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	res := b.String()
	if res == "" {
		return "package"
	}
	return res
}

// StreamLANPackage builds and streams a standard LAN Package ZIP into the provided io.Writer.
func StreamLANPackage(w io.Writer, opts PackageOptions) (*PackageMetadata, error) {
	zw := zip.NewWriter(w)
	defer zw.Close()

	metadata := &PackageMetadata{
		Name:             opts.Name,
		AccountID:        opts.AccountID,
		Created:          time.Now().Format(time.RFC3339),
		ContainsCfg:      false,
		ContainsUserdata: false,
		Files:            make([]PackageFileInfo, 0),
		Type:             "lan_package",
	}

	// 1. Add Userdata (730)
	if opts.IncludeUserdata && opts.UserdataDir != "" {
		if fi, err := os.Stat(opts.UserdataDir); err == nil && fi.IsDir() {
			var baseDir string
			var prefix string
			if filepath.Base(opts.UserdataDir) == "730" {
				baseDir = opts.UserdataDir
				prefix = "730/"
			} else if has730, _ := os.Stat(filepath.Join(opts.UserdataDir, "730")); has730 != nil && has730.IsDir() {
				baseDir = filepath.Join(opts.UserdataDir, "730")
				prefix = "730/"
			} else {
				baseDir = opts.UserdataDir
				prefix = "730/"
			}

			err = filepath.Walk(baseDir, func(path string, info os.FileInfo, walkErr error) error {
				if walkErr != nil || info.IsDir() {
					return walkErr
				}
				rel, err := filepath.Rel(baseDir, path)
				if err != nil {
					return err
				}
				zipEntryName := prefix + filepath.ToSlash(rel)
				infoEntry, err := writeFileToZip(zw, path, zipEntryName)
				if err != nil {
					return err
				}
				metadata.Files = append(metadata.Files, *infoEntry)
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed adding userdata to package: %w", err)
			}
			if len(metadata.Files) > 0 {
				metadata.ContainsUserdata = true
			}
		}
	}

	// 2. Add Game CFG
	if opts.IncludeCfg && opts.CfgDir != "" {
		if fi, err := os.Stat(opts.CfgDir); err == nil && fi.IsDir() {
			cfgCount := 0
			err = filepath.Walk(opts.CfgDir, func(path string, info os.FileInfo, walkErr error) error {
				if walkErr != nil || info.IsDir() {
					return walkErr
				}
				rel, err := filepath.Rel(opts.CfgDir, path)
				if err != nil {
					return err
				}
				zipEntryName := "cfg/" + filepath.ToSlash(rel)
				infoEntry, err := writeFileToZip(zw, path, zipEntryName)
				if err != nil {
					return err
				}
				metadata.Files = append(metadata.Files, *infoEntry)
				cfgCount++
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed adding cfg to package: %w", err)
			}
			if cfgCount > 0 {
				metadata.ContainsCfg = true
			}
		}
	}

	// 3. metadata.json
	metaBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed marshalling metadata: %w", err)
	}
	metaWriter, err := zw.Create("metadata.json")
	if err != nil {
		return nil, fmt.Errorf("failed creating metadata.json in zip: %w", err)
	}
	if _, err := metaWriter.Write(metaBytes); err != nil {
		return nil, fmt.Errorf("failed writing metadata.json: %w", err)
	}

	// 4. restore.bat
	batContent := GenerateRestoreBat(metadata.ContainsCfg, metadata.ContainsUserdata, metadata.AccountID)
	batWriter, err := zw.Create("restore.bat")
	if err != nil {
		return nil, fmt.Errorf("failed creating restore.bat in zip: %w", err)
	}
	if _, err := batWriter.Write([]byte(batContent)); err != nil {
		return nil, fmt.Errorf("failed writing restore.bat: %w", err)
	}

	return metadata, nil
}

func writeFileToZip(zw *zip.Writer, srcPath, zipName string) (*PackageFileInfo, error) {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}
	defer srcFile.Close()

	w, err := zw.Create(zipName)
	if err != nil {
		return nil, err
	}

	hasher := sha256.New()
	mw := io.MultiWriter(w, hasher)

	bufPtr := copyBufPool.Get().(*[]byte)
	defer copyBufPool.Put(bufPtr)

	copied, err := io.CopyBuffer(mw, srcFile, *bufPtr)
	if err != nil {
		return nil, err
	}

	return &PackageFileInfo{
		Path:   zipName,
		Size:   copied,
		SHA256: hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

// CreateLANPackage writes a LAN Package ZIP to disk according to opts.
func CreateLANPackage(opts PackageOptions) (string, error) {
	outPath := opts.OutputPath
	if outPath == "" {
		ts := time.Now().Format("20060102_150405")
		safeName := SanitizeFilename(opts.Name)
		outPath = filepath.Join("backup", "lan_packages", fmt.Sprintf("CS2Backup_%s_%s.zip", safeName, ts))
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", fmt.Errorf("failed creating output directory: %w", err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("failed creating zip package file: %w", err)
	}
	defer f.Close()

	if _, err := StreamLANPackage(f, opts); err != nil {
		_ = os.Remove(outPath)
		return "", err
	}

	return outPath, nil
}

// isZipSlip checks whether a zip file entry poses a Zip-Slip directory traversal risk.
func isZipSlip(entryName string) bool {
	clean := filepath.Clean(entryName)
	// Must not escape parent
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, "/../") || strings.Contains(clean, "\\..\\") {
		return true
	}
	// Must not be absolute
	if filepath.IsAbs(clean) || strings.HasPrefix(entryName, "/") || strings.HasPrefix(entryName, "\\") {
		return true
	}
	// Must not contain Windows drive designator
	if len(clean) >= 2 && clean[1] == ':' {
		return true
	}
	return false
}

// ValidatePackage inspects a LAN Package ZIP for Zip-Slip vulnerabilities and verifies all file checksums.
func ValidatePackage(zipPath string) (*PackageValidationResult, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	var metadataFile *zip.File
	var restoreBatFile *zip.File
	fileMap := make(map[string]*zip.File)

	for _, f := range r.File {
		if isZipSlip(f.Name) {
			return nil, fmt.Errorf("zip-slip path traversal attempt detected in entry: %s", f.Name)
		}
		normName := filepath.ToSlash(f.Name)
		fileMap[normName] = f
		if strings.HasSuffix(normName, "metadata.json") {
			metadataFile = f
		}
		if strings.HasSuffix(normName, "restore.bat") {
			restoreBatFile = f
		}
	}

	if metadataFile == nil {
		return nil, fmt.Errorf("missing metadata.json in LAN Package")
	}

	// Read and parse metadata.json
	metaRc, err := metadataFile.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata.json: %w", err)
	}
	defer metaRc.Close()

	var metadata PackageMetadata
	if err := json.NewDecoder(metaRc).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode metadata.json: %w", err)
	}

	result := &PackageValidationResult{
		Valid:         true,
		AllValid:      true,
		HasRestoreBat: restoreBatFile != nil,
		Metadata:      metadata,
		Validation:    make([]FileValidationItem, 0, len(metadata.Files)),
	}

	// Determine prefix if metadata is under subfolder
	prefix := ""
	metaNorm := filepath.ToSlash(metadataFile.Name)
	if idx := strings.LastIndex(metaNorm, "metadata.json"); idx > 0 {
		prefix = metaNorm[:idx]
	}

	for _, fi := range metadata.Files {
		lookupKey := fi.Path
		if prefix != "" && !strings.HasPrefix(lookupKey, prefix) {
			lookupKey = prefix + fi.Path
		}
		zf, found := fileMap[lookupKey]
		if !found {
			// Try without prefix
			zf, found = fileMap[fi.Path]
		}

		if !found {
			result.AllValid = false
			result.Validation = append(result.Validation, FileValidationItem{
				Path:     fi.Path,
				Valid:    false,
				Expected: fi.SHA256,
				Error:    "file missing in zip",
			})
			continue
		}

		rc, err := zf.Open()
		if err != nil {
			result.AllValid = false
			result.Validation = append(result.Validation, FileValidationItem{
				Path:     fi.Path,
				Valid:    false,
				Expected: fi.SHA256,
				Error:    err.Error(),
			})
			continue
		}

		hasher := sha256.New()
		_, _ = io.Copy(hasher, rc)
		rc.Close()

		actualHash := hex.EncodeToString(hasher.Sum(nil))
		valid := actualHash == fi.SHA256
		if !valid {
			result.AllValid = false
		}
		result.Validation = append(result.Validation, FileValidationItem{
			Path:     fi.Path,
			Valid:    valid,
			Expected: fi.SHA256,
			Actual:   actualHash,
		})
	}

	return result, nil
}
