//go:build !windows

package appcleanup

func RecordApplicationUser() error       { return nil }
func RegisteredUserSID() (string, error) { return "", ErrMaintenanceUnsupported }
