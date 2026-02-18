# My (@bigolu) .envrc is not at the root of the repo so we can't always use $PWD
REPO_ROOT="${REPO_ROOT:-$PWD}"

for hook in unload pre_load post_load; do
    declare -n hook_env_var="DIRENV_HOOK_${hook^^}_fish"
    export hook_env_var="
        source $(shell_escape "$REPO_ROOT/demo/autocomplete-plugin/hooks.fish")
        _complete_fish_$hook
    "
done

for hook in unload pre_load post_load; do
    declare -n hook_env_var="DIRENV_HOOK_${hook^^}_bash"
    export hook_env_var="
        echo This is the $hook hook for bash. Currently, autocomplete is unimplemented.
    "
done