//go:build windows

package appcleanup

import (
	"errors"
	"runtime"

	"golang.org/x/sys/windows"
)

func currentUserIdentity() (UserIdentity, error) {
	token := windows.GetCurrentProcessToken()
	tokenUser, err := token.GetTokenUser()
	if err != nil || tokenUser == nil || tokenUser.User.Sid == nil {
		return UserIdentity{}, ErrIdentityUnavailable
	}
	account, _, _, err := tokenUser.User.Sid.LookupAccount("")
	if err != nil {
		return UserIdentity{}, err
	}
	profile, err := token.GetUserProfileDirectory()
	if err != nil {
		return UserIdentity{}, err
	}
	appData, err := windows.KnownFolderPath(windows.FOLDERID_RoamingAppData, 0)
	if err != nil || !pathWithin(profile, appData) {
		return UserIdentity{}, ErrIdentityUnavailable
	}
	return UserIdentity{SID: tokenUser.User.Sid.String(), Account: account, ProfileDir: profile, AppDataDir: appData}, nil
}

func AcquireApplicationLock() (*ApplicationLock, error) { return acquireCurrentActivity(false) }
func AcquireMaintenanceLock() (*ApplicationLock, error) { return acquireCurrentActivity(true) }
func acquireCurrentActivity(exclusive bool) (*ApplicationLock, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return acquireActivityLock(`Global\AtlasNote.Activity.v2.`+user.User.Sid.String(), exclusive)
}

// Apps share an activity marker; only maintenance is exclusive. Individual
// storage writer locks continue to decide which spaces may run concurrently.
func acquireActivityLock(scope string, exclusive bool) (*ApplicationLock, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	name, err := windows.UTF16PtrFromString(scope + ".gate")
	if err != nil {
		return nil, err
	}
	gate, err := windows.CreateMutex(nil, false, name)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return nil, err
	}
	defer windows.CloseHandle(gate)
	state, err := windows.WaitForSingleObject(gate, 5000)
	if err != nil || (state != windows.WAIT_OBJECT_0 && state != windows.WAIT_ABANDONED) {
		return nil, ErrApplicationRunning
	}
	defer windows.ReleaseMutex(gate)
	own, other := ".apps", ".maintenance"
	if exclusive {
		own, other = other, own
	}
	probeName, _ := windows.UTF16PtrFromString(scope + other)
	probe, probeErr := windows.CreateEvent(nil, 1, 0, probeName)
	if probe != 0 {
		_ = windows.CloseHandle(probe)
	}
	if probeErr != nil {
		return nil, errors.Join(ErrApplicationRunning, probeErr)
	}
	ownName, _ := windows.UTF16PtrFromString(scope + own)
	h, err := windows.CreateEvent(nil, 1, 0, ownName)
	if err != nil && !(errors.Is(err, windows.ERROR_ALREADY_EXISTS) && !exclusive) {
		if h != 0 {
			_ = windows.CloseHandle(h)
		}
		return nil, errors.Join(ErrApplicationRunning, err)
	}
	return &ApplicationLock{release: func() error { return windows.CloseHandle(h) }}, nil
}

type ApplicationLock struct{ release func() error }

func (lock *ApplicationLock) Release() error {
	if lock == nil || lock.release == nil {
		return nil
	}
	release := lock.release
	lock.release = nil
	return release()
}
func pathWithin(root, candidate string) bool {
	return containsPath(root, candidate)
}
