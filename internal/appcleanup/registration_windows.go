//go:build windows

package appcleanup

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const registrationBase = `Software\AtlasNote\Installations\`

var wtsQuerySessionInformation = windows.NewLazySystemDLL("wtsapi32.dll").NewProc("WTSQuerySessionInformationW")

func sessionUserSID() (string, error) {
	var session uint32
	if err := windows.ProcessIdToSessionId(uint32(os.Getpid()), &session); err != nil || session == 0 {
		return "", ErrIdentityUnavailable
	}
	query := func(class uintptr) (string, error) {
		var buffer *uint16
		var size uint32
		ok, _, err := wtsQuerySessionInformation.Call(0, uintptr(session), class, uintptr(unsafe.Pointer(&buffer)), uintptr(unsafe.Pointer(&size)))
		if ok == 0 {
			return "", err
		}
		defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(buffer)))
		if buffer == nil || size < 2 || size > 65536 || size%2 != 0 {
			return "", ErrIdentityUnavailable
		}
		return windows.UTF16ToString(unsafe.Slice(buffer, size/2)), nil
	}
	user, err := query(5) // WTSUserName
	if err != nil || user == "" {
		return "", ErrIdentityUnavailable
	}
	domain, err := query(7) // WTSDomainName
	if err != nil || domain == "" {
		return "", ErrIdentityUnavailable
	}
	sid, _, kind, err := windows.LookupSID("", domain+`\`+user)
	if err != nil || kind != windows.SidTypeUser {
		return "", ErrIdentityUnavailable
	}
	return sid.String(), nil
}

func productionRegistration() userRegistration {
	return userRegistration{
		current: currentUserIdentity, sessionSID: sessionUserSID,
		elevated: func() bool { return windows.GetCurrentProcessToken().IsElevated() },
		read: func(key string) (string, error) {
			k, err := registry.OpenKey(registry.CURRENT_USER, registrationBase+key, registry.QUERY_VALUE)
			if err != nil {
				return "", err
			}
			defer k.Close()
			sid, _, err := k.GetStringValue("UserSID")
			return sid, err
		},
		write: func(key, sid string) error {
			k, _, err := registry.CreateKey(registry.CURRENT_USER, registrationBase+key, registry.SET_VALUE)
			if err != nil {
				return err
			}
			defer k.Close()
			return k.SetStringValue("UserSID", sid)
		},
	}
}
func RecordApplicationUser() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return productionRegistration().record(executable)
}
func RegisteredUserSID() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return productionRegistration().target(executable)
}
