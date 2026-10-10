# Security

## Reporting

Email **dev@ariavesta.com**, or open a [private advisory](https://github.com/Botropolis-City/botropolis/security/advisories/new).
Please do not open a public issue for anything exploitable.

Expect an acknowledgement within a week.
This is one person's side project, so a fix may take longer than that, and I would rather tell you honestly than promise a schedule I cannot keep.

## Supported versions

The latest release.
There is no back-porting.

## What Botropolis touches

Worth knowing before you audit it, and worth knowing before you install it:

**It reads `~/.claude` and never writes to it.**
Session records, transcripts and the usage cache are read;
nothing under `~/.claude` is modified, with one deliberate exception — `botropolis install-hooks` edits `~/.claude/settings.json` to add the hook, keeps a backup beside it, and `--remove` takes it out again.

**Transcripts are private and stay local.**
They contain whatever you have typed into a session.
Botropolis draws them on your own machine, sends them nowhere, and has no network client at all.

**The daemon listens on a unix socket**, by default under `$XDG_RUNTIME_DIR`, which is user-owned and not reachable from the network.
There is no TCP listener and no HTTP server.

**Sessions are started through the `claude` CLI**, never by writing to its state directly.
`botropolis new` shells out to `claude --bg`;
the shell integration in `packaging/botropolis.bash` wraps the `claude` command in your interactive shell and falls through to the real CLI for anything it does not handle.

**The usage probe is fenced.**
Refreshing usage runs `claude -p "/usage"`, which writes a transcript.
That runs inside a directory the session loader skips, and its transcripts are pruned, so reading usage cannot make Botropolis see itself as a session.

## What would count as a vulnerability

Anything that gets transcript content off the machine, anything that writes to `~/.claude` outside `install-hooks`, anything reachable over the network, and any path where a crafted transcript or hook payload leads to code execution.
