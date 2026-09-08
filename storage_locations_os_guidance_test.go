package main

import (
	"fmt"
	"syscall"
	"testing"
)

type storageOSCauseCase struct {
	err            syscall.Errno
	reason, action string
}

func TestOSGuidancePlatformBoundary(t *testing.T) {
	for _, tc := range storageOSCauseCases() {
		reason, _ := storageLocationOSGuidance(fmt.Errorf("wrapped: %w", tc.err))
		if tc.reason == "書き込み不可" {
			if reason != "" {
				t.Fatalf("unknown errno %d classified as %q", tc.err, reason)
			}
		} else if reason != tc.reason {
			t.Fatalf("errno %d: %q", tc.err, reason)
		}
	}
}
