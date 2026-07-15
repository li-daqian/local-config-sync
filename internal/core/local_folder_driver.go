package core

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sort"
)

var localFolderCapabilities = DriverCapabilities{History: false, ConditionalWrite: true, AtomicPublish: true}

type LocalFolderDriver struct{}

func (d *LocalFolderDriver) Prepare(repository Repository) error {
	return os.MkdirAll(repository.WorkspacePath, 0o755)
}

func snapshotRevision(snapshot map[string]FileSnapshot) string {
	keys := make([]string, 0, len(snapshot))
	for key := range snapshot {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	hash := sha256.New()
	for _, key := range keys {
		_, _ = hash.Write([]byte(key + ":" + snapshot[key].SHA256 + "|"))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (d *LocalFolderDriver) Inspect(context DriverContext) (RepositoryStatus, error) {
	snapshot, err := SnapshotDirectory(context.Repository.WorkspacePath, "")
	if err != nil {
		return RepositoryStatus{}, err
	}
	return RepositoryStatus{State: "synced", RemoteRevision: snapshotRevision(snapshot), LocalChanges: []string{}, Capabilities: localFolderCapabilities}, nil
}

func (d *LocalFolderDriver) Pull(context DriverContext) (PullResult, error) {
	status, err := d.Inspect(context)
	return PullResult{RemoteRevision: status.RemoteRevision}, err
}
func (d *LocalFolderDriver) Push(context DriverContext, _ string) (PushResult, error) {
	status, err := d.Inspect(context)
	return PushResult{RemoteRevision: status.RemoteRevision}, err
}

func (d *LocalFolderDriver) Doctor(repository Repository) (DiagnosticResult, error) {
	if err := d.Prepare(repository); err != nil {
		return DiagnosticResult{}, err
	}
	file, err := os.CreateTemp(repository.WorkspacePath, ".local-config-write-check-")
	if err == nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}
	check := DiagnosticCheck{Name: "folder-access", OK: err == nil}
	if check.OK {
		check.Message = "Local folder is readable and writable"
	} else {
		check.Message = "Local folder is not readable and writable"
		check.Remediation = "Check mount status and filesystem permissions"
	}
	return DiagnosticResult{OK: check.OK, Checks: []DiagnosticCheck{check}}, nil
}
