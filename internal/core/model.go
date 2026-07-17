package core

type LinkMode string

const (
	LinkModeSymlink LinkMode = "symlink"
	LinkModeCopy    LinkMode = "copy"
)

type MappingKind string

const (
	MappingKindDirectory MappingKind = "directory"
	MappingKindFile      MappingKind = "file"
)

type InitialStrategy string

const (
	InitialStrategyAuto   InitialStrategy = "auto"
	InitialStrategyLocal  InitialStrategy = "local"
	InitialStrategyRemote InitialStrategy = "remote"
)

type GlobalConfig struct {
	Version             int            `yaml:"version" json:"version"`
	DefaultRepositoryID string         `yaml:"defaultRepositoryId,omitempty" json:"defaultRepositoryId,omitempty"`
	DefaultLinkMode     LinkMode       `yaml:"defaultLinkMode" json:"defaultLinkMode"`
	AutoSync            AutoSyncConfig `yaml:"autoSync" json:"autoSync"`
}

type AutoSyncConfig struct {
	Enabled         bool `yaml:"enabled" json:"enabled"`
	DebounceSeconds int  `yaml:"debounceSeconds" json:"debounceSeconds"`
}

type DriverCapabilities struct {
	History          bool `json:"history"`
	ConditionalWrite bool `json:"conditionalWrite"`
	AtomicPublish    bool `json:"atomicPublish"`
}

type RepositoryOptions struct {
	RemoteURL string `yaml:"remoteUrl,omitempty" json:"remoteUrl,omitempty"`
	Branch    string `yaml:"branch,omitempty" json:"branch,omitempty"`
	Path      string `yaml:"path,omitempty" json:"path,omitempty"`
}

type Repository struct {
	ID            string            `yaml:"id" json:"id"`
	Name          string            `yaml:"name" json:"name"`
	Type          string            `yaml:"type" json:"type"`
	WorkspacePath string            `yaml:"workspacePath" json:"workspacePath"`
	CredentialRef string            `yaml:"credentialRef,omitempty" json:"credentialRef,omitempty"`
	Options       RepositoryOptions `yaml:"options" json:"options"`
}

type SafeRepository struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Type                 string            `json:"type"`
	WorkspacePath        string            `json:"workspacePath"`
	Options              RepositoryOptions `json:"options"`
	CredentialConfigured bool              `json:"credentialConfigured"`
}

func SanitizeRepository(repository Repository) SafeRepository {
	return SafeRepository{
		ID: repository.ID, Name: repository.Name, Type: repository.Type,
		WorkspacePath: repository.WorkspacePath, Options: repository.Options,
		CredentialConfigured: repository.CredentialRef != "",
	}
}

type RepositoryRegistryFile struct {
	Version      int          `yaml:"version"`
	Repositories []Repository `yaml:"repositories"`
}

type Mapping struct {
	ID           string      `yaml:"id" json:"id"`
	ProjectPath  string      `yaml:"projectPath" json:"projectPath"`
	ProjectName  string      `yaml:"projectName" json:"projectName"`
	RepositoryID string      `yaml:"repositoryId" json:"repositoryId"`
	SourcePath   string      `yaml:"sourcePath" json:"sourcePath"`
	TargetPath   string      `yaml:"targetPath" json:"targetPath"`
	Mode         LinkMode    `yaml:"mode" json:"mode"`
	Kind         MappingKind `yaml:"kind,omitempty" json:"kind"`
	CreatedAt    string      `yaml:"createdAt" json:"createdAt"`
	UpdatedAt    string      `yaml:"updatedAt" json:"updatedAt"`
}

type MappingRegistryFile struct {
	Version  int       `yaml:"version"`
	Mappings []Mapping `yaml:"mappings"`
}

type FileSnapshot struct {
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
	Deleted bool   `json:"deleted,omitempty"`
}

type StateError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Time    string    `json:"time"`
}

type RepositoryState struct {
	Version        int                     `json:"version"`
	RepositoryID   string                  `json:"repositoryId"`
	RemoteRevision string                  `json:"remoteRevision,omitempty"`
	LastSyncTime   string                  `json:"lastSyncTime,omitempty"`
	Files          map[string]FileSnapshot `json:"files"`
	LastError      *StateError             `json:"lastError,omitempty"`
}

type DiagnosticCheck struct {
	Name        string `json:"name"`
	OK          bool   `json:"ok"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

type DiagnosticResult struct {
	OK     bool              `json:"ok"`
	Checks []DiagnosticCheck `json:"checks"`
}

type RepositoryStatus struct {
	State          string             `json:"state"`
	RemoteRevision string             `json:"remoteRevision,omitempty"`
	LocalChanges   []string           `json:"localChanges"`
	RemoteChanged  bool               `json:"remoteChanged"`
	Capabilities   DriverCapabilities `json:"capabilities"`
}

type RepositorySummary struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Type           string             `json:"type"`
	State          string             `json:"state"`
	WorkspacePath  string             `json:"workspacePath"`
	RemoteRevision string             `json:"remoteRevision,omitempty"`
	Capabilities   DriverCapabilities `json:"capabilities"`
}

type MappingSummary struct {
	ID                string      `json:"id"`
	RepositoryID      string      `json:"repositoryId"`
	SourcePath        string      `json:"sourcePath"`
	TargetPath        string      `json:"targetPath"`
	Mode              LinkMode    `json:"mode"`
	Kind              MappingKind `json:"kind"`
	MappedFiles       []string    `json:"mappedFiles"`
	ExcludeConfigured bool        `json:"excludeConfigured"`
}

type FileStatusSummary struct {
	MappingID    string `json:"mappingId"`
	RepositoryID string `json:"repositoryId"`
	LocalPath    string `json:"localPath"`
	RemotePath   string `json:"remotePath"`
	Status       string `json:"status"`
	LocalExists  bool   `json:"localExists"`
	RemoteExists bool   `json:"remoteExists"`
}

type FileDiff struct {
	MappingID       string `json:"mappingId"`
	RepositoryID    string `json:"repositoryId"`
	LocalPath       string `json:"localPath"`
	RemotePath      string `json:"remotePath"`
	RemoteRevision  string `json:"remoteRevision"`
	LocalExists     bool   `json:"localExists"`
	RemoteExists    bool   `json:"remoteExists"`
	ContentEncoding string `json:"contentEncoding"`
	LocalContent    string `json:"localContent,omitempty"`
	RemoteContent   string `json:"remoteContent,omitempty"`
}

type ConflictStrategy string

const (
	ConflictStrategyLocal  ConflictStrategy = "local"
	ConflictStrategyRemote ConflictStrategy = "remote"
)

type MappingPreview struct {
	State              string      `json:"state"`
	Kind               MappingKind `json:"kind"`
	SourcePath         string      `json:"sourcePath"`
	TargetPath         string      `json:"targetPath"`
	SourceAbsolutePath string      `json:"sourceAbsolutePath"`
	TargetAbsolutePath string      `json:"targetAbsolutePath"`
	SourceExists       bool        `json:"sourceExists"`
	TargetExists       bool        `json:"targetExists"`
	SensitivePaths     []string    `json:"sensitivePaths"`
}

type RepositoryFileList struct {
	RepositoryID string   `json:"repositoryId"`
	Files        []string `json:"files"`
}

type GitHubRepository struct {
	NameWithOwner string `json:"nameWithOwner"`
	Private       bool   `json:"private"`
	SSHURL        string `json:"sshUrl"`
	URL           string `json:"url"`
	DefaultBranch string `json:"defaultBranch"`
}

type SyncResult struct {
	RepositoryID   string `json:"repositoryId"`
	State          string `json:"state"`
	RemoteRevision string `json:"remoteRevision,omitempty"`
	LastSyncTime   string `json:"lastSyncTime,omitempty"`
}

type StatusResult struct {
	ProjectPath  string              `json:"projectPath"`
	State        string              `json:"state"`
	Repositories []RepositorySummary `json:"repositories"`
	Mappings     []MappingSummary    `json:"mappings"`
	Files        []FileStatusSummary `json:"files"`
	LastSyncTime string              `json:"lastSyncTime,omitempty"`
}
