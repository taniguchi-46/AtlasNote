//go:build windows

package localipc

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestGeneratedDescriptorDACLAllowsOnlyCurrentUser(t *testing.T) {
	server := startTestServer(t, ServerConfig{ManagementRoot: t.TempDir(), StorageSpaceID: "test-space"})
	descriptor, err := windows.GetNamedSecurityInfo(
		server.descriptorPath,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatal(err)
	}
	currentUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || currentUser == nil || currentUser.User.Sid == nil {
		t.Fatal("resolve current Windows user")
	}
	owner, _, err := descriptor.Owner()
	if err != nil || owner == nil || !owner.Equals(currentUser.User.Sid) {
		t.Fatal("descriptor owner is not the current Windows user")
	}
	control, _, err := descriptor.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("descriptor DACL is not protected from inherited entries")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil {
		t.Fatalf("read descriptor DACL: %v", err)
	}
	if dacl.AceCount != 1 {
		t.Fatalf("descriptor DACL entry count = %d", dacl.AceCount)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || !aceSID.Equals(currentUser.User.Sid) {
		t.Fatalf("descriptor DACL contains an unexpected entry for %s", aceSID.String())
	}
}
