//go:build !windows

package appcleanup

func currentUserIdentity() (UserIdentity, error) {
	return UserIdentity{}, ErrMaintenanceUnsupported
}

func AcquireApplicationLock() (*ApplicationLock, error) {
	return &ApplicationLock{release: func() error { return nil }}, nil
}

type ApplicationLock struct {
	release func() error
}

func (lock *ApplicationLock) Release() error {
	if lock == nil || lock.release == nil {
		return nil
	}
	release := lock.release
	lock.release = nil
	return release()
}

func AcquireMaintenanceLock() (*ApplicationLock, error) { return AcquireApplicationLock() }
