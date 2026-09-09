package appcleanup

import (
	"errors"
	"strings"
)

func ParseMaintenanceRequest(args []string, targetSID func() (string, error)) (Request, error) {
	request := Request{}
	registered := false
	for _, arg := range args {
		switch arg {
		case "--registered-user":
			registered = true
		case "--delete-display-settings":
			request.DeleteDisplaySettings = true
		case "--delete-credentials":
			request.DeleteCredentials = true
		default:
			return request, errors.New("保守コマンドの指定が正しくありません。")
		}
	}
	if !registered || (!request.DeleteDisplaySettings && !request.DeleteCredentials) {
		return request, ErrIdentityUnavailable
	}
	sid, err := targetSID()
	if err != nil || strings.TrimSpace(sid) == "" {
		return request, ErrIdentityUnavailable
	}
	request.ExpectedSID = sid
	return request, nil
}
