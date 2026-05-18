// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-go-compile-instrumentation/tool/ex"
)

const (
	EnvOtelcWorkDir    = "OTELC_WORK_DIR"
	EnvOtelcRules      = "OTELC_RULES"
	EnvOtelcBuildFlags = "OTELC_BUILD_FLAGS"
	// EnvOtelcStats enables per-toolexec timing stats when set to "1".
	// Set automatically when --stats is used; propagated to child processes.
	EnvOtelcStats = "OTELC_STATS"
	// EnvOtelcDebug enables debug-level logging when set to "1".
	// Set automatically when --debug is used; propagated to child processes.
	EnvOtelcDebug = "OTELC_DEBUG"
	BuildTempDir  = ".otelc-build"
	OtelcRoot     = "github.com/open-telemetry/opentelemetry-go-compile-instrumentation"
)

func GetMatchedRuleFile() string {
	const matchedRuleFile = "matched.json"
	return GetBuildTemp(matchedRuleFile)
}

func GetBackupStateFile() string {
	const backupStateFile = "backup.json"
	return GetBuildTemp(backupStateFile)
}

// GetAddedImportsFileForProcess returns the per-process import tracking file.
// Each compile process writes to its own file to avoid inter-process race conditions.
func GetAddedImportsFileForProcess() string {
	pid := os.Getpid()
	return GetBuildTemp(fmt.Sprintf("added_imports.%d.json", pid))
}

// GetAddedImportsPattern returns the glob pattern for all import tracking files.
// Used by the link phase to discover and merge all per-process import files.
func GetAddedImportsPattern() string {
	return GetBuildTemp("added_imports.*.json")
}

func GetOtelcWorkDir() string {
	wd := os.Getenv(EnvOtelcWorkDir)
	if wd == "" {
		wd, _ = os.Getwd()
		return wd
	}
	return wd
}

// GetBuildTemp returns the path to the build temp directory $BUILD_TEMP/name
func GetBuildTempDir() string {
	return filepath.Join(GetOtelcWorkDir(), BuildTempDir)
}

// GetBuildTemp returns the path to the build temp directory $BUILD_TEMP/name
func GetBuildTemp(name string) string {
	return filepath.Join(GetOtelcWorkDir(), BuildTempDir, name)
}

func backupFilePath(path string) string {
	p := filepath.Clean(path)
	sum := sha256.Sum256([]byte(p))
	return hex.EncodeToString(sum[:])
}

// BackupFiles copies the specified files to the backup directory and records their paths in a state file for later restoration.
func BackupFiles(files ...string) error {
	var err error
	backupDir := GetBuildTemp("backup")
	for _, src := range files {
		dst := filepath.Join(backupDir, backupFilePath(src))
		err = ex.Join(err, CopyFile(src, dst))
	}
	if err != nil {
		return ex.Wrapf(err, "failed to copy backup files")
	}

	f := GetBackupStateFile()
	file, createErr := os.Create(f)
	if createErr != nil {
		return ex.Wrapf(createErr, "failed to create file %s", f)
	}
	defer file.Close()

	bs, marshalErr := json.Marshal(files)
	if marshalErr != nil {
		return ex.Wrapf(marshalErr, "failed to marshal backup state to JSON")
	}

	if _, writeErr := file.Write(bs); writeErr != nil {
		return ex.Wrapf(writeErr, "failed to write JSON to file %s", f)
	}

	return nil
}

// RestoreBackupFiles reads the backup state file to get the list of original file paths, then copies the backed-up files from the backup directory back to their original locations.
func RestoreBackupFiles() error {
	f := GetBackupStateFile()
	file, err := os.Open(f)
	if err != nil {
		return ex.Wrapf(err, "failed to open backup state file %s", f)
	}
	defer file.Close()

	var files []string
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(&files); err != nil {
		return ex.Wrapf(err, "failed to decode backup state JSON from file %s", f)
	}

	backupDir := GetBuildTemp("backup")
	for _, src := range files {
		dst := filepath.Join(backupDir, backupFilePath(src))
		err = ex.Join(err, CopyFile(dst, src))
	}

	return err
}

// GetBuildFlags returns the build flags from OTELC_BUILD_FLAGS environment variable.
// The flags are stored as a JSON-encoded string array to preserve arguments that contain spaces.
// Returns nil if not set or on decode error.
func GetBuildFlags() []string {
	encoded := os.Getenv(EnvOtelcBuildFlags)
	if encoded == "" {
		return nil
	}
	var flags []string
	if err := json.Unmarshal([]byte(encoded), &flags); err != nil {
		// Malformed JSON, return nil
		return nil
	}
	return flags
}

// EncodeBuildFlags encodes build flags as a JSON string for storage in an environment variable.
// This preserves arguments that contain spaces (e.g., -tags "foo bar").
func EncodeBuildFlags(flags []string) string {
	if len(flags) == 0 {
		return ""
	}
	encoded, err := json.Marshal(flags)
	if err != nil {
		return ""
	}
	return string(encoded)
}
