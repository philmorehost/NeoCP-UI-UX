package uzme

import (
	"context"
	"testing"
	"time"
)

func TestMigrationPipeline(t *testing.T) {
	mgr := NewMigrationManager()
	source := SourcePanelConfig{
		Hostname: "old.server.com",
		Username: "root",
	}

	progress := make(chan MigrationStream)
	ctx := context.Background()

	go mgr.ExecuteZeroDowntimeMigration(ctx, source, "test.com", progress)

	stages := []string{"syncing_files", "syncing_db", "proxy_active", "complete"}
	stageIdx := 0

	timeout := time.After(10 * time.Second)

	for {
		select {
		case p, ok := <-progress:
			if !ok {
				if stageIdx < len(stages) {
					t.Errorf("Progress closed early. Last stage: %s", stages[stageIdx])
				}
				return
			}
			t.Logf("Migration Stage: %s (%f%%)", p.Status, p.ProgressPct)
			if p.Status == stages[stageIdx] {
				stageIdx++
			}
		case <-timeout:
			t.Fatal("Migration test timed out")
		}
	}
}
