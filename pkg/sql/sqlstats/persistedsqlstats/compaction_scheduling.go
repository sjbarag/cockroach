// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package persistedsqlstats

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/jobs"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/scheduledjobs"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlutil"
	"github.com/cockroachdb/errors"
	pbtypes "github.com/gogo/protobuf/types"
)

const compactionScheduleName = "sql-stats-compaction"

// ErrDuplicatedSchedules indicates that there is already a schedule for sql
// stats compaction job existing in the system.scheduled_jobs table.
var ErrDuplicatedSchedules = errors.New("creating multiple sql stats compaction is disallowed")

// CreateSQLStatsCompactionScheduleIfNotYetExist registers SQL Stats compaction job with the
// scheduled job subsystem so the compaction job can be run periodically. This
// is done during the cluster startup migration.
func CreateSQLStatsCompactionScheduleIfNotYetExist(
	ctx context.Context, ie sqlutil.InternalExecutor, txn *kv.Txn, st *cluster.Settings,
) (*jobs.ScheduledJob, error) {
	scheduleExists, err := checkExistingCompactionSchedule(ctx, ie, txn)
	if err != nil {
		return nil, err
	}

	if scheduleExists {
		return nil, ErrDuplicatedSchedules
	}

	compactionSchedule := jobs.NewScheduledJob(scheduledjobs.ProdJobSchedulerEnv)

	schedule := SQLStatsCleanupRecurrence.Get(&st.SV)
	if err := compactionSchedule.SetSchedule(schedule); err != nil {
		return nil, err
	}

	compactionSchedule.SetScheduleDetails(jobspb.ScheduleDetails{
		Wait:    jobspb.ScheduleDetails_SKIP,
		OnError: jobspb.ScheduleDetails_RETRY_SCHED,
	})

	compactionSchedule.SetScheduleLabel(compactionScheduleName)
	compactionSchedule.SetOwner(username.NodeUserName())

	args, err := pbtypes.MarshalAny(&ScheduledSQLStatsCompactorExecutionArgs{})
	if err != nil {
		return nil, err
	}
	compactionSchedule.SetExecutionDetails(
		tree.ScheduledSQLStatsCompactionExecutor.InternalName(),
		jobspb.ExecutionArguments{Args: args},
	)

	compactionSchedule.SetScheduleStatus(string(jobs.StatusPending))
	if err = compactionSchedule.Create(ctx, ie, txn); err != nil {
		return nil, err
	}

	return compactionSchedule, nil
}

// CreateCompactionJob creates a system.jobs record.
// We do not need to worry about checking if the job already exist;
// at most 1 job semantics are enforced by scheduled jobs system.
func CreateCompactionJob(
	ctx context.Context, createdByInfo *jobs.CreatedByInfo, txn *kv.Txn, jobRegistry *jobs.Registry,
) (jobspb.JobID, error) {
	record := jobs.Record{
		Description: "automatic SQL Stats compaction",
		Username:    username.NodeUserName(),
		Details:     jobspb.AutoSQLStatsCompactionDetails{},
		Progress:    jobspb.AutoSQLStatsCompactionProgress{},
		CreatedBy:   createdByInfo,
	}

	jobID := jobRegistry.MakeJobID()
	if _, err := jobRegistry.CreateAdoptableJobWithTxn(ctx, record, jobID, txn); err != nil {
		return jobspb.InvalidJobID, err
	}
	return jobID, nil
}

func checkExistingCompactionSchedule(
	ctx context.Context, ie sqlutil.InternalExecutor, txn *kv.Txn,
) (exists bool, _ error) {
	query := "SELECT count(*) FROM system.scheduled_jobs WHERE schedule_name = $1"

	row, err := ie.QueryRowEx(ctx, "check-existing-sql-stats-schedule", txn,
		sessiondata.InternalExecutorOverride{User: username.NodeUserName()},
		query, compactionScheduleName,
	)

	if err != nil {
		return false /* exists */, err
	}

	if row == nil {
		return false /* exists */, errors.AssertionFailedf("unexpected empty result when querying system.scheduled_job")
	}

	if len(row) != 1 {
		return false /* exists */, errors.AssertionFailedf("unexpectedly received %d columns", len(row))
	}

	// Defensively check the count.
	return tree.MustBeDInt(row[0]) > 0, nil /* err */
}
