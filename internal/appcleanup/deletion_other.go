//go:build !windows

package appcleanup

type preparedDeletion struct{}

func prepareCacheDeletion(string) (*preparedDeletion, error) { return nil, ErrMaintenanceUnsupported }
func (*preparedDeletion) Close()                             {}
func (*preparedDeletion) Remove() error                      { return ErrMaintenanceUnsupported }
