package runtime

import "context"

// Fake is an in-memory Runtime for testing orchestration that starts or stops
// containers, without a real Podman. It records the specs it was asked to run.
type Fake struct {
	IsAvailable bool
	VersionStr  string
	Started     []RunSpec
	Stopped     []string
	NextID      string
	RunErr      error
	StopErr     error
}

func (f *Fake) Available() bool { return f.IsAvailable }

func (f *Fake) Version(context.Context) (string, error) { return f.VersionStr, nil }

func (f *Fake) Run(_ context.Context, spec RunSpec) (string, error) {
	if f.RunErr != nil {
		return "", f.RunErr
	}
	f.Started = append(f.Started, spec)
	id := f.NextID
	if id == "" {
		id = "fake-container"
	}
	return id, nil
}

func (f *Fake) Stop(_ context.Context, id string) error {
	if f.StopErr != nil {
		return f.StopErr
	}
	f.Stopped = append(f.Stopped, id)
	return nil
}
