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

// repositoryConfigGetData mirrors LoreRepositoryConfigGetEventData
// (lore-revision/src/repository.rs).
type repositoryConfigGetData struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CurrentUserID returns the identifier lock status reports as a lock's
// Owner (lock.go's Lock.Owner - "Identifier of the user that holds the
// lock" per LoreLockFileStatusEventData), so it's directly comparable
// against Owner to tell "locked by me" from "locked by someone else".
//
// Tries `lore auth info` first (lore-client/src/cli/commands/auth.rs's
// AuthCommands::Info: "omit for current user") - works when a real auth
// endpoint is configured. Falls back to the repo's configured commit
// identity (`lore repository config get identity`, .lore/config.toml's
// `identity` field) when that fails or comes back empty: lock acquire
// itself now uses that same identity as owner whenever no auth endpoint
// is available (see project_lazylore_lock_owner_todo memory), so without
// this fallback every one of your own locks on a no-auth server would
// compare against "" and misreport as someone else's.
func CurrentUserID(r Runner) (string, error) {
	if id, err := authInfoUserID(r); err == nil && id != "" {
		return id, nil
	}
	return configIdentity(r)
}

func authInfoUserID(r Runner) (string, error) {
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

func configIdentity(r Runner) (string, error) {
	res, err := runChecked(r, "repository", "config", "get", "identity")
	if err != nil {
		return "", err
	}
	events, err := parseEvents(res.Stdout)
	if err != nil {
		return "", fmt.Errorf("lore repository config get identity: %w", err)
	}
	for _, e := range events {
		if e.TagName != "repositoryConfigGet" {
			continue
		}
		var data repositoryConfigGetData
		if err := json.Unmarshal(e.Data, &data); err != nil {
			return "", fmt.Errorf("parsing repositoryConfigGet event: %w", err)
		}
		return data.Value, nil
	}
	return "", nil
}
