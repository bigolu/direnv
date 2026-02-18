package cmd

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

func supportedShellFormattedString() string {
	res := "["
	for k := range supportedShells {
		res += k + ", "
	}
	res = strings.TrimSuffix(res, ", ")
	res += "]"
	return res
}

var exportTypeNewShell = "new_shell"
var exportTypeExit = "exit"

// CmdExport is `direnv export $0`
var CmdExport = &Cmd{
	Name: "export",
	Desc: `Loads an .envrc or .env and prints the diff in terms of exports.
  Supported SHELL values are: ` + supportedShellFormattedString(),
	Args:    []string{"SHELL", "[--type]", "[" + exportTypeNewShell + " | " + exportTypeExit + "]"},
	Private: false,
	Action:  cmdWithWarnTimeout(actionWithConfig(exportCommand)),
}

func exportCommand(currentEnv Env, args []string, config *Config) (err error) {
	defer log.SetPrefix(log.Prefix())
	log.SetPrefix(log.Prefix() + "export:")
	logDebug("start")

	var target string
	if len(args) > 1 {
		target = args[1]
	}

	var exportType string
	if len(args) > 3 {
		exportType = args[3]
	}

	shell := DetectShell(target)
	if shell == nil {
		return fmt.Errorf("unknown target shell '%s'", target)
	}

	logDebug("loading RCs")
	loadedRC := config.LoadedRC()
	toLoad := findEnvUp(config.WorkDir, config.LoadDotenv)

	if loadedRC == nil && toLoad == "" {
		return
	}

	logDebug("updating RC")
	log.SetPrefix(log.Prefix() + "update:")

	logDebug("Determining action:")
	logDebug("toLoad: %#v", toLoad)
	logDebug("loadedRC: %#v", loadedRC)

	var currentEnvState *State
	currentEnvState, err = LoadStateFromString(currentEnv[DIRENV_STATE])
	if err != nil {
		return err
	}

	switch {
	case toLoad == "":
		logDebug("no RC found, unloading")
	case loadedRC == nil:
		logDebug("no RC (implies no DIRENV_DIFF),loading")
	case loadedRC.path != toLoad:
		logDebug("new RC, loading")
	case loadedRC.times.Check() != nil:
		logDebug("file changed, reloading")
	case currentEnv[DIRENV_REQUIRED] != "":
		// Force reload if required files were pending approval.
		// The approval status might have changed even if file times haven't.
		logDebug("required files pending, reloading")
	case exportType != "":
		shellWithHooks, ok := shell.(ShellWithHooks)
		if !ok {
			return
		}

		hooksToRun := map[string]string{}
		switch exportType {
		// t_exec, t_subshell
		case exportTypeNewShell:
			hooksToRun[HOOK_SET_PROCESS_MARKER] = "true"
			addToHooksToRun(currentEnvState, shellWithHooks, hooksToRun, HOOK_POST_LOAD)
		// t_exit
		case exportTypeExit:
			addToHooksToRun(currentEnvState, shellWithHooks, hooksToRun, HOOK_UNLOAD)
		}

		diffString, diffErr := shellWithHooks.ExportWithHooks(nil, nil, hooksToRun)
		if diffErr != nil {
			return fmt.Errorf("ToShellWithHooks() failed: %w", diffErr)
		}

		fmt.Print(diffString)

		return
	default:
		logDebug("no update needed")
		return
	}

	var previousEnv, newEnv Env
	var newEnvState *State

	if previousEnv, err = config.Revert(currentEnv); err != nil {
		err = fmt.Errorf("Revert() failed: %w", err)
		logDebug("err: %v", err)
		return
	}

	if toLoad == "" {
		logStatus(config, "unloading")
		newEnv = previousEnv.Copy()
		newEnv.CleanContext()
	} else {
		newEnv, err = config.EnvFromRC(toLoad, previousEnv)
		if err != nil {
			logDebug("err: %v", err)
			// If loading fails, fall through and deliver a diff anyway,
			// but still exit with an error.  This prevents retrying on
			// every prompt.
		}
		if newEnv == nil {
			// unless of course, the error was in hashing and timestamp loading,
			// in which case we have to abort because we don't know what timestamp
			// to put in the diff!
			return
		}

		newEnvState = MakeStateFromEnv(newEnv)
		newEnv[DIRENV_STATE] = newEnvState.ToString()
	}

	diffPreviousWithNew := previousEnv.Diff(newEnv)
	if out := diffStatus(diffPreviousWithNew); out != "" && !config.HideEnvDiff {
		logStatus(config, "export %s", out)
	}

	var diffString string
	var diffErr error
	if shellWithHooks, ok := shell.(ShellWithHooks); ok {
		hooksToRun := map[string]string{}
		if toLoad == "" {
			// t_cd_outside_direnv
			hooksToRun[HOOK_UNSET_PROCESS_MARKER] = "true"
			addToHooksToRun(currentEnvState, shellWithHooks, hooksToRun, HOOK_UNLOAD)
		} else {
			if loadedRC == nil {
				// t_cd_to_direnv
				hooksToRun[HOOK_SET_PROCESS_MARKER] = "true"
			} else {
				// t_change_watched_file, t_cd_to_a_different_direnv, t_block, t_allow
				addToHooksToRun(currentEnvState, shellWithHooks, hooksToRun, HOOK_UNLOAD)
			}

			// t_cd_to_direnv, t_change_watched_file, t_cd_to_a_different_direnv,
			// t_block, t_allow
			addToHooksToRun(newEnvState, shellWithHooks, hooksToRun, HOOK_PRE_LOAD)
			addToHooksToRun(newEnvState, shellWithHooks, hooksToRun, HOOK_POST_LOAD)
		}

		diffCurrentWithPrevious := currentEnv.Diff(previousEnv)
		diffString, diffErr = shellWithHooks.ExportWithHooks(diffCurrentWithPrevious.MakeShellExport(), diffPreviousWithNew.MakeShellExport(), hooksToRun)
	} else {
		diffCurrentWithNew := currentEnv.Diff(newEnv)
		diffString, diffErr = diffCurrentWithNew.ToShell(shell)
	}

	if diffErr != nil {
		return fmt.Errorf("ToShell[WithHooks]() failed: %w", diffErr)
	}
	logDebug("env diff %s", diffString)
	fmt.Print(diffString)

	return
}

// Return a string of +/-/~ indicators of an environment diff
func diffStatus(oldDiff *EnvDiff) string {
	if oldDiff.Any() {
		var out []string
		for key := range oldDiff.Prev {
			_, ok := oldDiff.Next[key]
			if !ok && !direnvKey(key) {
				out = append(out, "-"+key)
			}
		}

		for key := range oldDiff.Next {
			_, ok := oldDiff.Prev[key]
			if direnvKey(key) {
				continue
			}
			if ok {
				out = append(out, "~"+key)
			} else {
				out = append(out, "+"+key)
			}
		}

		sort.Strings(out)
		return strings.Join(out, " ")
	}
	return ""
}

func direnvKey(key string) bool {
	return strings.HasPrefix(key, "DIRENV_")
}

func addToHooksToRun(state *State, shellWithHooks ShellWithHooks, hooksToRun map[string]string, hookName string) {
	hook := state.Hooks.GetHook(hookName, shellWithHooks)
	if hook != "" {
		hooksToRun[hookName] = hook
	}
}
