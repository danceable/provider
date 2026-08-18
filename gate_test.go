package provider

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGate_RunsOnce(t *testing.T) {
	t.Parallel()

	g := newGate()

	var runs atomic.Int32
	run := func() error {
		runs.Add(1)

		return nil
	}

	for range 3 {
		require.NoError(t, g.do(run))
	}

	assert.Equal(t, int32(1), runs.Load())
}

func TestGate_ReportsTheRunError(t *testing.T) {
	t.Parallel()

	g := newGate()
	failure := errors.New("failed")

	// Every caller gets the error of the single run, and the run is not retried.
	var runs atomic.Int32
	run := func() error {
		runs.Add(1)

		return failure
	}

	require.ErrorIs(t, g.do(run), failure)
	require.ErrorIs(t, g.do(run), failure)
	assert.Equal(t, int32(1), runs.Load())
}

func TestGate_CallersWaitForTheRun(t *testing.T) {
	t.Parallel()

	g := newGate()

	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})

	var (
		wg   sync.WaitGroup
		seen atomic.Int32
	)

	wg.Add(1)
	go func() {
		defer wg.Done()

		assert.NoError(t, g.do(func() error {
			close(started)
			<-release
			close(finished)

			return nil
		}))
	}()

	// Only once the run is under way are the others guaranteed to be waiters.
	<-started

	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			assert.NoError(t, g.do(func() error { return errors.New("must not run") }))

			// The run had finished before this caller was released.
			select {
			case <-finished:
				seen.Add(1)
			default:
			}
		}()
	}

	close(release)
	wg.Wait()

	assert.Equal(t, int32(5), seen.Load())
}
