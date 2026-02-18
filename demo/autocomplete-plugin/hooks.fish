# For information about how fish autocomplete works:
# https://github.com/fish-shell/fish-shell/issues/8261
#
# A simpler way to implement this would be to append the autocomplete scripts to
# the variable $fish_complete_path when we enter the environment and remove them
# when we leave. However, when we remove a file from $fish_complete_path, _all_
# autocomplete entries for that command will be removed[1], even entries that
# weren't added by the file that we removed. Instead, we keep track of the
# individual autocomplete entries that get added to fish after `eval`ing the
# autocomplete file and when direnv unloads the environment, we remove only the
# entries that we added.
#
# [1]: https://github.com/fish-shell/fish-shell/blob/d01a403c65076f5ac34402c5201f75b95b95276f/src/complete.rs#L2549C24-L2552C1

function _complete_fish_unload
    _complete_fish_debug 'In unload'

    if test (count $_complete_fish_files) -eq 0
        return
    end

    _complete_fish_debug 'Removing completions for these files:'\n"$(string join \n $_complete_fish_files)"

    for file_index in (seq (count $_complete_fish_files))
        set -l added_entries (string split --no-empty \n "$_complete_fish_entries[$file_index]")
        set -l file $_complete_fish_files[$file_index]
        set -l command (path change-extension '' (path basename $file))
        set -l current_entries (complete $command)
        for entry in $added_entries
            if set -l entry_index (contains --index -- $entry $current_entries)
                set --erase current_entries[$entry_index]
            end
        end
        complete --erase $command
        printf %s\n $current_entries | source
    end
    
    set -l our_variables (set --names | string match --regex --groups-only '^(_complete_fish.*|COMPLETE_FISH.*)')
    set --erase $our_variables

    set -l our_functions (functions --names --all | string match --regex --groups-only '^(_complete_fish.*)')
    functions --erase $our_functions
end

function _complete_fish_pre_load
    _complete_fish_debug 'In pre load'

    _complete_fish_convert_xdg_to_path_variable

    # This variable holds the value of XDG_DATA_DIRS before the direnv environment is
    # loaded. It's exported to account for the following state transitions:
    #   - t_subshell: The XDG_DATA_DIRS in the sub shell will contain everything
    #     that direnv adds to it.
    #   - t_exec: The XDG_DATA_DIRS in the new process will contain everything
    #     that direnv adds to it.
    #
    # It needs to end in 'PATH' so fish can treat it like an array, but join it with
    # ':' when exported.
    if not set --query --global --export COMPLETE_FISH_OLD_XDG_PATH
        set --global --export COMPLETE_FISH_OLD_XDG_PATH $XDG_DATA_DIRS
    end
end

function _complete_fish_post_load
    _complete_fish_debug 'In post load'

    _complete_fish_convert_xdg_to_path_variable

    set -l xdg_files
    for dir in $XDG_DATA_DIRS
        # This was in XDG before direnv was loaded so ignore it
        if contains $dir $COMPLETE_FISH_OLD_XDG_PATH
            continue
        end

        set -l fish_dir $dir'/fish/vendor_completions.d'
        if not test -d $fish_dir
            continue
        end
        set --append xdg_files $fish_dir/*
    end
    if test (count $xdg_files) -eq 0
        return
    end

    _complete_fish_debug 'Adding completions for these files:'\n"$(string join \n $xdg_files)"
    _complete_fish_add $xdg_files
end

function _complete_fish_debug --argument-names message
    if test -n "$COMPLETE_FISH_DEBUG"
        echo "[completion-sync] $message" >&2
    end
end

function _complete_fish_add
    for file in $argv
        set -l command (path change-extension '' (path basename $file))

        set -l old_entries (complete $command)
        source $file
        set -l new_entries (complete $command)
        set -l added_entries
        for new_entry in $new_entries
            if not contains $new_entry $old_entries
                set --append added_entries $new_entry
            end
        end
        if test (count $added_entries) -eq 0
            _complete_fish_debug "This file didn't add any completion entries, ignoring: $file"
            continue
        end

        if set --query --global _complete_fish_files
            set --append _complete_fish_files $file
        else
            set --global _complete_fish_files $file
        end

        set -l added_entries_string "$(string join --no-empty \n $added_entries)"
        # The autocomplete entries for `ruff` exceeded the max length of an
        # environment variable so a global variable should be used.
        if set --query --global _complete_fish_entries
            set --append _complete_fish_entries $added_entries_string
        else
            set --global _complete_fish_entries $added_entries_string
        end
    end
end

function _complete_fish_convert_xdg_to_path_variable
    # TODO: On Linux, fish treats XDG_DATA_DIRS like a "path variable", meaning
    # its value is split on ":". However, on macOS it's a single element. I
    # think they should do the same thing on either platform.
    set --path XDG_DATA_DIRS "$XDG_DATA_DIRS"
end