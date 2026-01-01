# Pulse Dev Container Shell Configuration

# Better prompt showing git branch and mock mode
parse_git_branch() {
    git branch 2> /dev/null | sed -e '/^[^*]/d' -e 's/* \(.*\)/ (\1)/'
}

get_mock_status() {
    if [ -f /workspaces/pulse/mock.env ] && grep -q "PULSE_MOCK_MODE=true" /workspaces/pulse/mock.env 2>/dev/null; then
        echo " [MOCK]"
    fi
}

export PS1='\[\033[01;32m\]\u@pulse-dev\[\033[00m\]:\[\033[01;34m\]\w\[\033[33m\]$(parse_git_branch)\[\033[35m\]$(get_mock_status)\[\033[00m\]\$ '

# Useful aliases
alias pd='cd /workspaces/pulse && ./scripts/hot-dev.sh'
alias ptest='go test ./...'
alias plint='golangci-lint run ./...'
alias pfmt='gofmt -w -s .'
alias plog='tail -f /tmp/pulse-dev.log'
alias mock-on='cd /workspaces/pulse && npm run mock:on'
alias mock-off='cd /workspaces/pulse && npm run mock:off'
alias mock-edit='cd /workspaces/pulse && npm run mock:edit'

# Helpful shortcuts
alias ll='ls -lah'
alias gs='git status'
alias gp='git pull'
alias gc='git commit'

# Show helpful info on shell start
echo ""
echo "🚀 Pulse Dev Container"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Shortcuts:"
echo "  pd          - Start hot-reload dev server"
echo "  ptest       - Run all Go tests"
echo "  plint       - Run Go linter"
echo "  pfmt        - Format Go code"
echo "  plog        - View dev server logs"
echo "  mock-on/off - Toggle mock mode"
echo ""
echo "Debug: Press F5 in VS Code to start debugger"
echo "Tasks: Cmd+Shift+P → 'Tasks: Run Task'"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
