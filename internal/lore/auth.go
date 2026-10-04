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
// Selects the repository's configured account for `lore auth info`; an
// unqualified query can pick a different cached account.
// A server without an auth endpoint reports anonymous locks as "<unknown>".
// Other failures or empty auth results
// fall back to the repository's configured identity.
func CurrentUserID(r Runner) (string, error) {
	identity, configErr := configIdentity(r)
	if id, err := authInfoUserID(r, identity); err == nil && id != "" {
		return id, nil
	}
	return identity, configErr
}

func authInfoUserID(r Runner, identity string) (string, error) {
	args := []string{"auth", "info"}
	if identity != "" {
		args = append(args, "--identity="+identity)
	}
	res, err := runChecked(r, args...)
	if err != nil {
		if events, parseErr := parseEvents(res.Stdout); parseErr == nil {
			complete, completeErr := findComplete(events)
			if completeErr == nil && complete.Status == 9 && complete.Error.ErrorCode == 9 && complete.Error.Message == "Operation not supported: authentication requires a configured auth endpoint" {
				return "<unknown>", nil
			}
		}
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
