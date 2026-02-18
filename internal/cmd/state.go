package cmd

import (
	"regexp"

	"github.com/direnv/direnv/v2/gzenv"
)

type State struct {
	Hooks Hooks
}

func (state *State) ToString() string {
	return gzenv.Marshal(state)
}

func LoadStateFromString(s string) (*State, error) {
	if s != "" {
		state := new(State)
		err := gzenv.Unmarshal(s, state)
		if err != nil {
			return nil, err
		}
		return state, nil
	} else {
		return newState(), nil
	}
}

func MakeStateFromEnv(env Env) *State {
	state := newState()
	hooks := state.Hooks

	regex := regexp.MustCompile(DIRENV_HOOK_PREFIX + `(.+)_([^_]+)`)
	for envVarName, envVarValue := range env {
		matches := regex.FindStringSubmatch(envVarName)
		if matches == nil {
			continue
		}
		hookName := matches[1]
		shellName := matches[2]

		hooks.SetHook(hookName, envVarValue, shellName)
	}

	return state
}

func newState() *State {
	state := new(State)
	state.Hooks = make(Hooks)
	return state
}
