package task

import (
	"errors"
	"fmt"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/logging"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/status"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestManagedTask(t *testing.T) {
	prepCount := 0
	runCount := 0

	tl := logging.NewRootLogger("test")
	prep := func() error {
		require.Equal(t, status.Preparing, tl.Status().Type())
		prepCount++
		return nil
	}
	run := func() error {
		require.Equal(t, status.InProgress, tl.Status().Type())
		runCount++
		return nil
	}
	msgFunc := func() string {
		return fmt.Sprintf("Run %d", runCount)
	}
	mt := NewManagedTask(tl, prep, run)

	require.Equal(t, 0, prepCount)
	require.Equal(t, 0, runCount)
	require.Equal(t, status.NotStarted, tl.Status().Type())

	err := mt.Prepare()
	require.NoError(t, err)
	require.Equal(t, 1, prepCount)
	require.Equal(t, 0, runCount)
	require.Equal(t, status.Ready, tl.Status().Type())

	err = mt.Prepare()
	require.NoError(t, err)
	require.Equal(t, 2, prepCount)
	require.Equal(t, 0, runCount)
	require.Equal(t, status.Ready, tl.Status().Type())

	// Already prepped
	err = mt.Run(msgFunc)
	require.NoError(t, err)
	require.Equal(t, 2, prepCount)
	require.Equal(t, 1, runCount)
	require.Equal(t, status.Success, tl.Status().Type())
	require.Equal(t, "Run 1", tl.Status().Msg())

	// If you run, then it needs to re-prep
	err = mt.Run(msgFunc)
	require.NoError(t, err)
	require.Equal(t, 3, prepCount)
	require.Equal(t, 2, runCount)
	require.Equal(t, status.Success, tl.Status().Type())
	require.Equal(t, "Run 2", tl.Status().Msg())
}

func TestManagedTaskFailRun(t *testing.T) {
	prepCount := 0
	runCount := 0

	const errMsg = "foo"

	tl := logging.NewRootLogger("test")
	prep := func() error {
		require.Equal(t, status.Preparing, tl.Status().Type())
		prepCount++
		return nil
	}
	run := func() error {
		require.Equal(t, status.InProgress, tl.Status().Type())
		runCount++
		return errors.New(errMsg)
	}
	msgFunc := func() string {
		return fmt.Sprintf("Run %d", runCount)
	}
	mt := NewManagedTask(tl, prep, run)

	require.Equal(t, 0, prepCount)
	require.Equal(t, 0, runCount)
	require.Equal(t, status.NotStarted, tl.Status().Type())

	err := mt.Prepare()
	require.NoError(t, err)
	require.Equal(t, 1, prepCount)
	require.Equal(t, 0, runCount)
	require.Equal(t, status.Ready, tl.Status().Type())

	err = mt.Prepare()
	require.NoError(t, err)
	require.Equal(t, 2, prepCount)
	require.Equal(t, 0, runCount)
	require.Equal(t, status.Ready, tl.Status().Type())

	// Already prepped
	err = mt.Run(msgFunc)
	require.Error(t, err)
	require.Equal(t, 2, prepCount)
	require.Equal(t, 1, runCount)
	require.Equal(t, status.Failed, tl.Status().Type())
	require.Equal(t, errMsg, tl.Status().Msg())

	// If you run, then it needs to re-prep
	err = mt.Run(msgFunc)
	require.Error(t, err)
	require.Equal(t, 3, prepCount)
	require.Equal(t, 2, runCount)
	require.Equal(t, status.Failed, tl.Status().Type())
	require.Equal(t, errMsg, tl.Status().Msg())
}

func TestManagedTaskFailPrep(t *testing.T) {
	prepCount := 0
	runCount := 0

	const errMsg = "foo"

	tl := logging.NewRootLogger("test")
	prep := func() error {
		require.Equal(t, status.Preparing, tl.Status().Type())
		prepCount++
		return errors.New(errMsg)
	}
	run := func() error {
		require.Equal(t, status.InProgress, tl.Status().Type())
		runCount++
		return nil
	}
	msgFunc := func() string {
		return fmt.Sprintf("Run %d", runCount)
	}
	mt := NewManagedTask(tl, prep, run)

	require.Equal(t, 0, prepCount)
	require.Equal(t, 0, runCount)
	require.Equal(t, status.NotStarted, tl.Status().Type())

	err := mt.Prepare()
	require.Error(t, err)
	require.Equal(t, 1, prepCount)
	require.Equal(t, 0, runCount)
	require.Equal(t, status.Failed, tl.Status().Type())
	require.Equal(t, errMsg, tl.Status().Msg())

	err = mt.Prepare()
	require.Error(t, err)
	require.Equal(t, 2, prepCount)
	require.Equal(t, 0, runCount)
	require.Equal(t, status.Failed, tl.Status().Type())
	require.Equal(t, errMsg, tl.Status().Msg())

	// Already prepped
	err = mt.Run(msgFunc)
	require.Error(t, err)
	require.Equal(t, 3, prepCount)
	require.Equal(t, 0, runCount)
	require.Equal(t, status.Failed, tl.Status().Type())
	require.Equal(t, errMsg, tl.Status().Msg())

	// If you run, then it needs to re-prep
	err = mt.Run(msgFunc)
	require.Error(t, err)
	require.Equal(t, 4, prepCount)
	require.Equal(t, 0, runCount)
	require.Equal(t, status.Failed, tl.Status().Type())
	require.Equal(t, errMsg, tl.Status().Msg())
}
