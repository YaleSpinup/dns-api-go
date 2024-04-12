package api

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestRollback(t *testing.T) {
	// nil input
	var rbfuncs []rollbackFunc
	rollBack(&rbfuncs)

	// empty input
	rbfuncs = []rollbackFunc{}
	rollBack(&rbfuncs)

	// test rolling back
	v := []int{}
	for i := 0; i < 10; i++ {
		v = append(v, i)
		f := func(ctx context.Context) error {
			if len(v) == 0 {
				return nil
			}

			value := v[len(v)-1]
			index := len(v) - 1

			t.Logf("rolling back value %d with index %d", value, index)

			if value != index {
				t.Errorf("unexpected value %d for v index %d", value, index)
				return fmt.Errorf("unexpected value %d for v index %d", value, index)
			}

			v = v[:len(v)-1]
			return nil
		}

		rbfuncs = append(rbfuncs, f)
	}
	rollBack(&rbfuncs)

	// return an error
	f := func(ctx context.Context) error {
		return errors.New("boom")
	}
	rbfuncs = append(rbfuncs, f)
	rollBack(&rbfuncs)
}

func TestRetry(t *testing.T) {
	if err := retry(3, 2, 1*time.Millisecond, func() error {
		return errors.New("boom")
	}); err == nil {
		t.Error("expected error when exceeding retry attempts, got nil")
	}

	if err := retry(3, 2, 1*time.Millisecond, func() error {
		return nil
	}); err != nil {
		t.Errorf("unexpected error for successful retry, got %s", err)
	}

	tr := 0
	f := func() error {
		if tr >= 2 {
			return nil
		}

		tr++
		return fmt.Errorf("too small %d", tr)
	}

	if err := retry(3, 2, 1*time.Millisecond, f); err != nil {
		t.Errorf("unexpected error for successful retry, got %s", err)
	}
}
