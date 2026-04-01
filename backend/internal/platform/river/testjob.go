package river

import (
	"context"
	"log/slog"

	"github.com/riverqueue/river"
)

// TestJobArgs defines the arguments for a test job used to verify
// that the River queue is processing work correctly.
type TestJobArgs struct {
	Message string `json:"message"`
}

// Kind returns the job type identifier.
func (TestJobArgs) Kind() string { return "test_job" }

// TestJobWorker processes test jobs by logging the message.
type TestJobWorker struct {
	river.WorkerDefaults[TestJobArgs]
}

// Work executes the test job.
func (w *TestJobWorker) Work(ctx context.Context, job *river.Job[TestJobArgs]) error {
	slog.Info("test job executed", "message", job.Args.Message, "job_id", job.ID)
	return nil
}
