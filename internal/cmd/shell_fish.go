package cmd

import (
	"fmt"
	"strings"
)

type fish struct{}

// Fish adds support for the fish shell as a host
var Fish Shell = fish{}

const fishHook = `
	function __direnv_export_eval --on-event fish_prompt;
		# t_exec, t_subshell
		#
		# We assume that if DIRENV_STATE is set, then we're in a direnv envrionment. 
		# So if our process marker isn't set and DIRENV_STATE is, then the user must 
		# have used exec or started a subshell.
		if not set --query --global _direnv_loaded && set --query --export DIRENV_STATE;
			"{{.SelfPath}}" export fish --type new_shell | source;
		else;
			"{{.SelfPath}}" export fish | source;
		end;

		if test "$direnv_fish_mode" != "disable_arrow";
			function __direnv_cd_hook --on-variable PWD;
				if test "$direnv_fish_mode" = "eval_after_arrow";
					set -g __direnv_export_again 0;
				else;
					"{{.SelfPath}}" export fish | source;
				end;
			end;
		end;
	end;

	function __direnv_export_eval_2 --on-event fish_preexec;
		if set -q __direnv_export_again;
			set -e __direnv_export_again;
			"{{.SelfPath}}" export fish | source;
			echo;
		end;

		functions --erase __direnv_cd_hook;
	end;

	# t_exit
	function __direnv_exit --on-event fish_exit;
		"{{.SelfPath}}" export fish --type exit | source;
	end;
`

func (sh fish) Name() string {
	return "fish"
}

func (sh fish) Hook() (string, error) {
	return fishHook, nil
}

func (sh fish) ExportWithHooks(unload ShellExport, load ShellExport, hooksToRun map[string]string) (string, error) {
	var builder strings.Builder

	var processMarker = "_direnv_loaded"
	if _, ok := hooksToRun[HOOK_SET_PROCESS_MARKER]; ok {
		builder.WriteString("set --global " + processMarker + " true; ")
	}
	if _, ok := hooksToRun[HOOK_UNSET_PROCESS_MARKER]; ok {
		builder.WriteString("set --erase --global " + processMarker + " true; ")
	}

	if unloadHook, ok := hooksToRun[HOOK_UNLOAD]; ok {
		builder.WriteString("eval " + sh.Escape(unloadHook) + "; ")
	}

	if unload != nil {
		unloadExport, err := sh.Export(unload)
		if err != nil {
			return "", err
		}
		builder.WriteString(unloadExport)
	}

	if preLoadHook, ok := hooksToRun[HOOK_PRE_LOAD]; ok {
		builder.WriteString("eval " + sh.Escape(preLoadHook) + "; ")
	}

	if load != nil {
		loadExport, err := sh.Export(load)
		if err != nil {
			return "", err
		}
		builder.WriteString(loadExport)
	}

	if postLoadHook, ok := hooksToRun[HOOK_POST_LOAD]; ok {
		builder.WriteString("eval " + sh.Escape(postLoadHook) + ";")
	}

	return builder.String(), nil
}

func (sh fish) Export(e ShellExport) (string, error) {
	var out string
	for key, value := range e {
		if value == nil {
			out += sh.unset(key)
		} else {
			out += sh.export(key, *value)
		}
	}
	return out, nil
}

func (sh fish) Dump(env Env) (string, error) {
	var out string
	for key, value := range env {
		out += sh.export(key, value)
	}
	return out, nil
}

func (sh fish) Escape(str string) string {
	in := []byte(str)
	out := "'"
	i := 0
	l := len(in)

	hex := func(char byte) {
		out += fmt.Sprintf("'\\X%02x'", char)
	}

	backslash := func(char byte) {
		out += string([]byte{BACKSLASH, char})
	}

	escaped := func(str string) {
		out += "'" + str + "'"
	}

	literal := func(char byte) {
		out += string([]byte{char})
	}

	for i < l {
		char := in[i]
		switch {
		case char == TAB:
			escaped(`\t`)
		case char == LF:
			escaped(`\n`)
		case char == CR:
			escaped(`\r`)
		case char <= US:
			hex(char)
		case char == SINGLE_QUOTE:
			backslash(char)
		case char == BACKSLASH:
			backslash(char)
		case char <= TILDE:
			literal(char)
		case char == DEL:
			hex(char)
		default:
			hex(char)
		}
		i++
	}

	out += "'"

	return out
}

func (sh fish) export(key, value string) string {
	if key == "PATH" {
		command := "set -x -g PATH"
		for _, path := range strings.Split(value, ":") {
			command += " " + sh.Escape(path)
		}
		return command + ";"
	}
	return "set -x -g " + sh.Escape(key) + " " + sh.Escape(value) + ";"
}

func (sh fish) unset(key string) string {
	return "set -e -g " + sh.Escape(key) + ";"
}
