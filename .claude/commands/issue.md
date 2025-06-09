Work with GitHub issues - list them and optionally fix one.

If $ARGUMENTS is provided and looks like a number, I'll fix that specific issue.
Otherwise, I'll list all open issues so you can pick one to work on.

Steps:
1. List current open issues with `gh issue list`
2. If you provide an issue number as $ARGUMENTS, I'll automatically fix it
3. If no arguments or non-numeric arguments, just show the list for you to browse

Usage:
- `/project:issue` - shows all open issues
- `/project:issue 123` - fixes issue #123
- `/project:issue bug` - shows issues (filters by "bug" label)