#!/usr/bin/env bash
# Printed once after install. Botropolis needs three things switched on that a
# package cannot switch on for you: a user daemon, the hooks, and the shell
# integration that makes a bare `claude` a background session. `botropolis
# setup` does the first two and prints the third.
set -e

cat <<'HINT'

Botropolis is installed. To finish, as yourself rather than root:

  botropolis setup

It starts the daemon, registers the hooks, and prints the one line to add to
your shell's startup file (~/.bashrc, ~/.zshrc or ~/.config/fish/config.fish).

Then `botropolis` opens the city, and `botropolis doctor` says what is missing.

HINT
