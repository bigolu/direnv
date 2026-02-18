package cmd

import (
	"regexp"
	"strings"
)

// map[shellName -> map[hookName -> hook]]
type Hooks map[string]map[string]string

var shellEscape = "direnv-shell-escape"
var shellEscapePrefix = "{{{" + shellEscape + " "
var shellEscapeSuffix = " " + shellEscape + "}}}"
var shellEscapeRegex = regexp.MustCompile(shellEscapePrefix + `.*?` + shellEscapeSuffix)

func (hooks Hooks) GetHook(hookName string, shellWithHooks ShellWithHooks) string {
	hooksForShell := hooks[shellWithHooks.Name()]
	if hooksForShell != nil {
		// PERF: Rather than process the escapes in all the hooks ahead of time,
		// we process them lazily here
		escapedHook := processEscapes(hooksForShell[hookName], shellWithHooks)
		return escapedHook
	}

	return ""
}

func (hooks Hooks) SetHook(hookName string, hook string, shellName string) {
	hooksForShell := hooks[shellName]
	if hooksForShell == nil {
		hooksForShell = map[string]string{}
		hooks[shellName] = hooksForShell
	}

	hooksForShell[hookName] = hook
}

func processEscapes(hook string, shellWithHooks ShellWithHooks) string {
	return shellEscapeRegex.ReplaceAllStringFunc(hook, func(match string) string {
		matchWithoutPrefix := strings.TrimPrefix(match, shellEscapePrefix)
		matchWithoutPrefixAndSuffix := strings.TrimSuffix(matchWithoutPrefix, shellEscapeSuffix)
		return shellWithHooks.Escape(matchWithoutPrefixAndSuffix)
	})
}
