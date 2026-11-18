// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	runnerv1 "gitea.dev/actionslib/runner/v1"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestGetRunningTaskByTokenRejectsCompletedCachedTask verifies that a token
// cached while a task is running stops authenticating after the task finishes.
func TestGetRunningTaskByTokenRejectsCompletedCachedTask(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	task, _ := newRunningTaskForCancelling(t, "cached-token-after-completion", true)
	task.GenerateAndFillToken()
	_, err := db.GetEngine(ctx).ID(task.ID).
		Cols("token_hash", "token_salt", "token_last_eight").
		Update(task)
	require.NoError(t, err)

	cacheKey := "actions:" + task.Token
	auth_model.TokenCache().Remove(cacheKey)
	defer auth_model.TokenCache().Remove(cacheKey)

	// First lookup takes the DB path and populates the successful-token cache.
	authenticated, err := GetRunningTaskByToken(ctx, task.Token)
	require.NoError(t, err)
	require.Equal(t, task.ID, authenticated.ID)

	// Finalize through the production state-transition path.
	_, err = UpdateTaskByState(ctx, task.RunnerID, &runnerv1.TaskState{
		Id: task.ID,
		Result: runnerv1.Result_RESULT_SUCCESS,
		StoppedAt: timestamppb.Now(),
	})
	require.NoError(t, err)

	// A terminal task must no longer authenticate, even while its cache entry exists.
	authenticated, err = GetRunningTaskByToken(ctx, task.Token)
	require.Error(t, err, "completed task token must be rejected")
	require.Nil(t, authenticated)
}
