//go:build integration

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSpreadWorkloadPreservesScopeAndDeadlineBounds(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err = conn.Exec(ctx, `CREATE TEMP TABLE scheduled_jobs(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),workspace_id uuid,kind text,state text,due_at timestamptz)`); err != nil {
		t.Fatal(err)
	}
	const workspace = "00000000-0000-4000-8000-000000000001"
	first := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	original := first.Add(time.Hour)
	for _, count := range []int{0, 1, 5} {
		if _, err = conn.Exec(ctx, `TRUNCATE scheduled_jobs;`); err != nil {
			t.Fatal(err)
		}
		if _, err = conn.Exec(ctx, `INSERT INTO scheduled_jobs(workspace_id,kind,state,due_at) SELECT $1,'timer','SCHEDULED',$3 FROM generate_series(1,$2::integer)`, workspace, count, original); err != nil {
			t.Fatal(err)
		}
		if _, err = conn.Exec(ctx, `INSERT INTO scheduled_jobs(workspace_id,kind,state,due_at) VALUES(gen_random_uuid(),'timer','SCHEDULED',$2),($1,'timer','CLAIMED',$2),($1,'node','SCHEDULED',$2)`, workspace, original); err != nil {
			t.Fatal(err)
		}
		tag, err := conn.Exec(ctx, timerSpreadUpdate, workspace, first, 60)
		if err != nil || tag.RowsAffected() != int64(count) {
			t.Fatalf("changed wrong timer set: %v %v", tag, err)
		}
		rows, err := conn.Query(ctx, `SELECT due_at FROM scheduled_jobs WHERE workspace_id=$1 AND kind='timer' AND state='SCHEDULED' ORDER BY due_at,id`, workspace)
		if err != nil {
			t.Fatal(err)
		}
		index := 0
		for rows.Next() {
			var due time.Time
			if err = rows.Scan(&due); err != nil {
				t.Fatal(err)
			}
			expected := first
			if count > 1 {
				expected = first.Add(time.Duration(index) * 60 * time.Second / time.Duration(count-1))
			}
			if !due.Equal(expected) {
				t.Fatalf("timer %d/%d: %s expected %s", index, count, due, expected)
			}
			index++
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		if index != count {
			t.Fatal("missing timers", index, count)
		}
		var untouched int
		if err = conn.QueryRow(ctx, `SELECT count(*) FROM scheduled_jobs WHERE due_at=$1`, original).Scan(&untouched); err != nil || untouched != 3 {
			t.Fatalf("unrelated work changed: %d %v", untouched, err)
		}
	}
}
