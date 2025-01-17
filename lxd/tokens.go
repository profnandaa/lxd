package main

import (
	"context"
	"time"

	"github.com/lxc/lxd/lxd/db/operationtype"
	"github.com/lxc/lxd/lxd/operations"
	"github.com/lxc/lxd/lxd/task"
	"github.com/lxc/lxd/shared/logger"
)

func autoRemoveExpiredTokens(ctx context.Context, d *Daemon) error {
	expiredTokenOps := make([]*operations.Operation, 0)

	for _, op := range operations.Clone() {
		// Only consider token operations
		if op.Type() != operationtype.ClusterJoinToken && op.Type() != operationtype.CertificateAddToken {
			continue
		}

		// Instead of cancelling the operation here, we add it to a list of expired token operations.
		// This allows us to only show log messages if there are expired tokens.
		expiry, ok := op.Metadata()["expiresAt"].(time.Time)
		if ok && time.Now().After(expiry) {
			expiredTokenOps = append(expiredTokenOps, op)
		}
	}

	if len(expiredTokenOps) == 0 {
		return nil
	}

	opRun := func(op *operations.Operation) error {
		for _, op := range expiredTokenOps {
			_, err := op.Cancel()
			if err != nil {
				logger.Debug("Failed removing expired token", logger.Ctx{"err": err, "id": op.ID()})
			}
		}

		return nil
	}

	op, err := operations.OperationCreate(d.State(), "", operations.OperationClassTask, operationtype.RemoveExpiredTokens, nil, nil, opRun, nil, nil, nil)
	if err != nil {
		logger.Error("Failed to start remove expired tokens operation", logger.Ctx{"err": err})
		return err
	}

	logger.Info("Removing expired tokens")

	err = op.Start()
	if err != nil {
		logger.Error("Failed to remove expired tokens", logger.Ctx{"err": err})
		return err
	}

	_, _ = op.Wait(ctx)

	logger.Debug("Done removing expired tokens")

	return nil
}

func autoRemoveExpiredTokensTask(d *Daemon) (task.Func, task.Schedule) {
	f := func(ctx context.Context) {
		_ = autoRemoveExpiredTokens(ctx, d)
	}

	return f, task.Every(time.Minute)
}
