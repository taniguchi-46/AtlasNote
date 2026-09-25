//go:build windows

package localipc

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func restrictDescriptor(path string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return errors.New("resolve current Windows user")
	}
	sddl := fmt.Sprintf("D:P(A;;GA;;;%s)", user.User.Sid.String())
	descriptor, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return errors.New("build IPC descriptor ACL")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return errors.New("read IPC descriptor ACL")
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	); err != nil {
		return errors.New("restrict IPC descriptor ACL")
	}
	return nil
}

func validateDescriptorSecurity(path string, _ os.FileInfo) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return errors.New("resolve current Windows user")
	}
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil || descriptor == nil {
		return errors.New("inspect IPC descriptor ACL")
	}
	owner, _, err := descriptor.Owner()
	if err != nil || owner == nil || !owner.Equals(user.User.Sid) {
		return errors.New("IPC descriptor owner does not match the current user")
	}
	return nil
}
