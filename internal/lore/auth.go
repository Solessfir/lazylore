package lore

import (
	"encoding/json"
	"fmt"
)

// authUserInfoData mirrors LoreAuthUserInfoEventData
// (lore-revision/src/auth/userinfo.rs).
type authUserInfoData struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CurrentUserID returns the authenticated user's identity via `lore auth
// info` with no user IDs given (lore-client/src/cli/commands/auth.rs's
// AuthCommands::Info: "omit for current user"). This is the same identifier
// lock status reports as a lock's Owner (lock.go's Lock.Owner - "Identifier
// of the user that holds the lock" per LoreLockFileStatusEventData), so
// it's directly comparable against Owner to tell "locked by me" from
// "locked by someone else".
func CurrentUserID(r Runner) (string, error) {
	res, err := runChecked(r, "auth", "info")
	if err != nil {
		return "", err
	}
	events, err := parseEvents(res.Stdout)
	if err != nil {
		return "", fmt.Errorf("lore auth info: %w", err)
	}
	for _, e := range events {
		if e.TagName != "authUserInfo" {
			continue
		}
		var data authUserInfoData
		if err := json.Unmarshal(e.Data, &data); err != nil {
			return "", fmt.Errorf("parsing authUserInfo event: %w", err)
		}
		return data.ID, nil
	}
	return "", nil
}
