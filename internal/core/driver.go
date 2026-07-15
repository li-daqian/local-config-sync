package core

type DriverContext struct {
	Repository       Repository
	Scopes           []string
	ExpectedRevision string
}

type PullResult struct {
	RemoteRevision string
	Changed        bool
}
type PushResult struct {
	RemoteRevision string
	Changed        bool
}

type RepositoryDriver interface {
	Prepare(Repository) error
	Inspect(DriverContext) (RepositoryStatus, error)
	Pull(DriverContext) (PullResult, error)
	Push(DriverContext, string) (PushResult, error)
	Doctor(Repository) (DiagnosticResult, error)
}
