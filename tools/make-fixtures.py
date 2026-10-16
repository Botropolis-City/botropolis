#!/usr/bin/env python3
"""Copy a scrubbed slice of ~/.claude into testing/helpers/fixtures/<name>/.

The slice is every session with a live record in ~/.claude/sessions, the N most
recently touched transcripts, the smallest bridge-session stub, the most
recent session that spawned subagents, and a pair of sessions that gives the
map a road — one reaching into the other's repo, both ends included, because a
road with no far end is dropped when the city is built. Structure, ids, timestamps, and `usage`
are kept verbatim; prompt text, tool inputs and outputs, thinking, titles,
slugs, file snapshots, and account identifiers are replaced with deterministic
placeholders so the fixture is stable across regenerations and safe to commit.

Layout written under the fixture directory:

  home/.claude/sessions/<pid>.json
  home/.claude/projects/<cwd>/<sid>.jsonl
  home/.claude/projects/<cwd>/<sid>/subagents/*.jsonl, *.meta.json
  home/.claude/stats-cache.json
  home/.claude.json                 (mcpServers only)
  manifest.json
  raw/...                           (unscrubbed copy, gitignored, --no-raw to skip)

Read-only against ~/.claude; stdlib only.
"""
import argparse
import hashlib
import json
import os
import re
import shutil
import sys
from pathlib import Path

HOME = Path.home()
REPO = Path(__file__).resolve().parent.parent
UUID_RE = re.compile(r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", re.I)

KEEP_KEYS = {
    "type", "subtype", "role", "model", "id", "uuid", "parentUuid", "leafUuid",
    "logicalParentUuid", "sessionId", "session_id", "messageId", "snapshotMessageId",
    "requestId", "tool_use_id", "toolUseID", "toolUseId", "sourceToolUseID",
    "sourceToolAssistantUUID", "interruptedMessageId", "promptId", "timestamp",
    "version", "entrypoint", "userType",
    "sessionKind", "effort", "perTurnEffort", "permissionMode", "mode", "agentType",
    "stop_reason", "stopReason", "service_tier", "kind",
    "status", "nameSource", "messagingSocketPath", "jobId", "startTime",
    "lastComputedDate", "date", "firstSessionDate", "operation", "trigger", "level",
    "scope", "direction", "origin", "promptSource",
    "queueOrigin", "teamName", "cronKind", "atis", "agentSetting", "toolDenialKind",
    "procStart", "startedAt", "updatedAt", "statusUpdatedAt", "nameSince", "until",
    "peerFeatures", "ts", "fallbackModel", "originalModel", "apiRefusalCategory", "modelId",
}
TOOL_PART_TYPES = {"tool_use", "server_tool_use", "mcp_tool_use"}
FORCE_SCRUB_KEYS = {"bridgeSessionId", "pidDomain", "ownerAccountUuid", "ownerOrganizationUuid"}

# Work context is renamed rather than kept or blanked. A district IS a cwd, a road
# is one session's cwd reaching into another's repo, and a card carries the branch
# and the PR's repository -- so these have to keep their shape, their nesting and
# their distinctness while losing the names, which belong to an employer and are
# not this project's to publish. Blanking them would change the city; keeping them
# publishes an org chart. See roadmap item 72 in the Botropolis-City meta repository.
MAPPED_PATH_KEYS = {"cwd", "relocatedCwd"}
MAPPED_SLUG_KEYS = {"gitBranch"}
MAPPED_REPO_KEYS = {"prRepository"}
MAPPED_URL_KEYS = {"prUrl"}
# MCP servers are radio towers and attribution is the beam between one and a
# building, so these drive what is drawn and cannot simply be blanked -- but the
# names are the vendor and the employer's plugin namespace. Mapped, shape kept.
MAPPED_MCP_KEYS = {"attributionMcpServer", "attributionMcpTool",
                   "attributionSkill", "attributionPlugin"}

# Maps whose KEYS are content rather than field names: feature flags are the
# employer's, and answers and annotations are keyed by the prompt that asked
# them. Everything else keyed oddly is load-bearing and must survive verbatim --
# modelUsage and stats-cache.json are keyed by model id, and renaming those would
# take the freight trains and the cost card with them.
CONTENT_KEYED_MAPS = {"featureFlags": "alias", "answers": "blank", "annotations": "blank"}

# Components that carry no employer information and must survive so the paths keep
# their shape: filesystem furniture, forge names, and this project's own repos.
SAFE_COMPONENTS = {
    "", "home", "workspaces", "github", "gitlab", "bitbucket", "aur", "tmp", "var",
    "opt", "srv", "mnt", "media", "usr", "etc", "root", "configuration", "src",
    "auroq", "botropolis", "bot-crossing", "botropolis-git", "bot-crossing-git",
}
ADJECTIVES = (
    "amber", "brisk", "copper", "dusty", "eager", "fallow", "gilded", "hollow",
    "ivory", "jagged", "keen", "level", "muted", "narrow", "ochre", "placid",
    "quiet", "russet", "slate", "tidal", "umber", "vivid", "waxen", "yonder",
    "azure", "brindle", "cinder", "drifting", "ember", "flinty", "granite", "hazel",
)
NOUNS = (
    "anvil", "basin", "cedar", "delta", "estuary", "furrow", "gantry", "harbour",
    "inlet", "junction", "kiln", "lantern", "meadow", "nettle", "orchard", "pylon",
    "quarry", "ridge", "sawmill", "thicket", "upland", "vault", "weir", "yard",
    "aqueduct", "bellows", "causeway", "dovecote", "foundry", "granary", "hedgerow", "ironworks",
)
# A rename has to preserve what a name DOES, not only that it is distinct, and
# these four are load-bearing in three separate ways that a hashed alias cannot
# satisfy by chance. Each was found by running the suite against the renamed
# tree, one after the other, never by reading the mapping:
#
#   length -- pkg/ui asserts plate padding, and a plate is sized from its text;
#   sort order -- pkg/claude and pkg/state assert output ordered by project;
#   hue -- pkg/city asserts a renamed project differs in hue from botropolis,
#          whose path does NOT change, and projectHue is FNV-1a over the full
#          path modulo three. So distinct names are not enough, and the target
#          is fixed: the alias has to miss one specific value. Twice I solved
#          this against the wrong pair by assuming which projects the test
#          compared instead of reading it.
#
# These four aliases are the same lengths as their originals, in the same
# lexical order, and give the same hue distinction. Solved rather than picked.
PINNED_ALIASES = {
    "cinders": "cinders",
    "ledger": "ledger",
    "mCedar": "mCedar",
    "mullet": "mullet",
}
_pseudonyms = {}


def pseudonym(word):
    """A stable, path-shaped alias for one component. Same input, same output, always."""
    if word in SAFE_COMPONENTS or not word:
        return word
    if word in PINNED_ALIASES:
        return PINNED_ALIASES[word]
    if word in _pseudonyms:
        return _pseudonyms[word]
    digest = hashlib.sha1(("botropolis-fixture/" + word).encode("utf-8")).hexdigest()
    # camelCase rather than hyphenated, because these same names are substituted
    # into source as well as into data, and some of the originals are Go
    # identifiers -- pkg/city/city_test.go declares `cinders` and `mullet` as
    # variables. A hyphen there is a syntax error, which is how this was found.
    head = ADJECTIVES[int(digest[:8], 16) % len(ADJECTIVES)]
    tail = NOUNS[int(digest[8:16], 16) % len(NOUNS)]
    alias = head + tail[:1].upper() + tail[1:]
    clash = [w for w, a in _pseudonyms.items() if a == alias and w != word]
    if clash:
        # Deterministic and loud rather than silently merging two districts into one.
        alias = alias + digest[16:20].upper()
    _pseudonyms[word] = alias
    return alias


def map_path(path):
    """Rename every component that names an employer, keeping depth and nesting."""
    if not isinstance(path, str) or not path:
        return path
    lead = "/" if path.startswith("/") else ""
    parts = [pseudonym(c) for c in path.strip("/").split("/")]
    return lead + "/".join(parts) + ("/" if path.endswith("/") and len(path) > 1 else "")


def map_repo(repo):
    """owner/name, kept as two components so a PR card still reads like one."""
    if not isinstance(repo, str) or "/" not in repo:
        return pseudonym(repo) if isinstance(repo, str) else repo
    return "/".join(pseudonym(c) for c in repo.split("/"))


def map_url(url):
    if not isinstance(url, str):
        return url
    m = re.match(r"^(https?://[^/]+/)(.+?)(/pull/\d+.*)?$", url)
    if not m:
        return placeholder(url)
    return m.group(1) + map_repo(m.group(2)) + (m.group(3) or "")


def map_slug(slug):
    """A branch name: one token, kept token-shaped."""
    if not isinstance(slug, str) or not slug:
        return slug
    return pseudonym(slug)


def map_qualified(name):
    """plugin:skill, server, or server_tool -- each part mapped, separators kept."""
    if not isinstance(name, str) or not name:
        return name
    for sep in (":", "__"):
        if sep in name:
            return sep.join(pseudonym(part) for part in name.split(sep))
    return pseudonym(name)


def map_tool_name(name):
    """mcp__<server>__<tool>, which pkg/claude parses, so the shape has to survive."""
    parts = name.split("__")
    if len(parts) < 3 or parts[0] != "mcp":
        return pseudonym(name)
    return "__".join(["mcp"] + [pseudonym(p) for p in parts[1:]])


def placeholder(s):
    if s == "":
        return ""
    digest = hashlib.sha1(s.encode("utf-8", "replace")).hexdigest()
    if UUID_RE.match(s):
        h = digest[:32]
        return f"{h[:8]}-{h[8:12]}-{h[12:16]}-{h[16:20]}-{h[20:32]}"
    return f"<scrubbed:{digest[:12]}>"


def scrub(obj, key=None, parent_type=None):
    if isinstance(obj, dict):
        own_type = obj.get("type") if isinstance(obj.get("type"), str) else parent_type
        out = {}
        for k, v in obj.items():
            # Some maps are keyed BY an absolute path -- trackedFileBackups in a
            # transcript, projects in .claude.json -- so a scrub that only ever
            # rewrites values renames nothing in them. Keys carry the employer's
            # names just as plainly as values do.
            kk = k
            if isinstance(k, str):
                if "/" in k:
                    kk = map_path(k)
                elif key in CONTENT_KEYED_MAPS:
                    kk = pseudonym(k) if CONTENT_KEYED_MAPS[key] == "alias" else placeholder(k)
            out[kk] = scrub(v, k, None if k == "input" else own_type)
        return out
    if isinstance(obj, list):
        return [scrub(v, key, parent_type) for v in obj]
    if isinstance(obj, str):
        if key in FORCE_SCRUB_KEYS:
            return placeholder(obj)
        if key in MAPPED_PATH_KEYS:
            return map_path(obj)
        if key in MAPPED_REPO_KEYS:
            return map_repo(obj)
        if key in MAPPED_URL_KEYS:
            return map_url(obj)
        if key in MAPPED_SLUG_KEYS:
            return map_slug(obj)
        if key in MAPPED_MCP_KEYS:
            return map_qualified(obj)
        if key == "name" and parent_type in TOOL_PART_TYPES and obj.startswith("mcp__"):
            return map_tool_name(obj)
        if key in KEEP_KEYS:
            return obj
        if key == "name" and parent_type in TOOL_PART_TYPES:
            return obj
        if key == "input_schema":
            return obj
        return placeholder(obj)
    return obj


def scrub_jsonl(src, dst):
    dst.parent.mkdir(parents=True, exist_ok=True)
    with open(src, encoding="utf-8", errors="replace") as fin, open(dst, "w", encoding="utf-8") as fout:
        for line in fin:
            complete = line.endswith("\n")
            line = line.rstrip("\n").lstrip("\x00")
            if not line:
                continue
            try:
                rec = json.loads(line)
            except json.JSONDecodeError:
                if not complete:
                    # A live transcript's last line is still being written.
                    continue
                fout.write(placeholder(line) + "\n")
                continue
            fout.write(json.dumps(scrub(rec), separators=(",", ":"), ensure_ascii=False) + "\n")


def scrub_json_file(src, dst, fn=scrub):
    dst.parent.mkdir(parents=True, exist_ok=True)
    with open(src, encoding="utf-8") as f:
        data = json.load(f)
    with open(dst, "w", encoding="utf-8") as f:
        json.dump(fn(data), f, separators=(",", ":"), ensure_ascii=False)
        f.write("\n")


def scrub_session_record(rec):
    out = scrub(rec)
    if isinstance(rec.get("name"), str):
        out["name"] = placeholder(rec["name"])
    for former in out.get("formerNames") or []:
        if isinstance(former, dict) and isinstance(former.get("name"), str):
            former["name"] = placeholder(former["name"])
    return out


def scrub_mcp_server(cfg):
    out = {}
    for k, v in cfg.items():
        if k in ("type", "command"):
            out[k] = v
        elif k == "url" and isinstance(v, str):
            # The host is the vendor. Keeping it published which observability and
            # ticketing stack the machine talks to, which is the employer's shape
            # again, so only the scheme survives.
            out[k] = re.sub(r"^([a-z]+)://[^/?#@]+.*$", r"\1://<scrubbed>/<scrubbed>", v)
        elif k == "args" and isinstance(v, list):
            out[k] = ["<scrubbed>" for _ in v]
        elif isinstance(v, dict):
            out[k] = {kk: "<scrubbed>" for kk in v}
        else:
            out[k] = "<scrubbed>"
    return out


def scrub_claude_json(data):
    out = {"mcpServers": {pseudonym(n): scrub_mcp_server(c) for n, c in (data.get("mcpServers") or {}).items()}}
    projects = {}
    for cwd, proj in (data.get("projects") or {}).items():
        servers = proj.get("mcpServers") if isinstance(proj, dict) else None
        if servers:
            projects[map_path(cwd)] = {"mcpServers": {pseudonym(n): scrub_mcp_server(c) for n, c in servers.items()}}
    out["projects"] = projects
    return out


def is_bridge_stub(path):
    try:
        with open(path, encoding="utf-8") as f:
            lines = [ln for ln in f.read().splitlines() if ln.strip()]
    except OSError:
        return False
    if len(lines) != 1:
        return False
    try:
        return json.loads(lines[0]).get("type") == "bridge-session"
    except json.JSONDecodeError:
        return False


def session_cwd(path):
    """The directory a session ran in.

    Scanned for rather than read off the first record: a transcript opens
    with mode, permission-mode and bridge-session lines that carry no
    cwd, so taking the first record's gives every session the empty
    string and quietly makes every later question about repos
    unanswerable.
    """
    try:
        with open(path, encoding="utf-8") as f:
            for line in f:
                if '"cwd"' not in line:
                    continue
                try:
                    cwd = json.loads(line).get("cwd")
                except json.JSONDecodeError:
                    continue
                if cwd:
                    return cwd
    except OSError:
        pass
    return ""


def cross_repo_touches(path, roots, own):
    """Files this session touched inside somebody else's repo, by repo.

    This is the fixture's only way to grow a road. pkg/state.Roads draws
    one when a session working in repo A touches a file under repo B, and
    pkg/city drops it again unless B also has a district — so counting
    the far end here, rather than just the crossings, is what makes the
    difference between a fixture with roads and one that merely looks as
    though it should have them.
    """
    others = sorted((r for r in roots if r and r != own), key=len, reverse=True)
    if not others:
        return {}
    found = {}
    try:
        with open(path, encoding="utf-8") as f:
            for line in f:
                if '"file_path"' not in line:
                    continue
                try:
                    rec = json.loads(line)
                except json.JSONDecodeError:
                    continue
                msg = rec.get("message") or {}
                for block in msg.get("content") or []:
                    if not isinstance(block, dict) or block.get("type") != "tool_use":
                        continue
                    fp = (block.get("input") or {}).get("file_path")
                    if not isinstance(fp, str) or not fp.startswith("/"):
                        continue
                    for root in others:
                        if fp.startswith(root + "/"):
                            found[root] = found.get(root, 0) + 1
                            break
    except OSError:
        return {}
    return found


def select_transcripts(claude_dir, live_ids, recent, exclude_id=None):
    projects = claude_dir / "projects"
    all_main = [p for p in projects.glob("*/*.jsonl") if p.stem != exclude_id and p.stat().st_size > 0]
    by_id = {p.stem: p for p in all_main}
    chosen = {}
    for sid in live_ids:
        if sid in by_id:
            chosen[sid] = by_id[sid]
    by_mtime = sorted(all_main, key=lambda p: p.stat().st_mtime, reverse=True)
    for p in by_mtime:
        if len([s for s in chosen if s not in live_ids]) >= recent:
            break
        chosen.setdefault(p.stem, p)
    stubs = sorted((p for p in all_main if is_bridge_stub(p)), key=lambda p: p.stat().st_size)
    if stubs:
        chosen.setdefault(stubs[0].stem, stubs[0])
    with_subagents = [p for p in by_mtime if (p.parent / p.stem / "subagents").is_dir()]
    if with_subagents:
        chosen.setdefault(with_subagents[0].stem, with_subagents[0])
    add_a_road(by_mtime, live_ids, chosen)
    return chosen


def add_a_road(by_mtime, live_ids, chosen):
    """Make sure the fixture has at least one road on it.

    Roads are the one thing on the map the recent-and-live rules cannot
    be relied on to produce, and the reason is worth writing down because
    it is not obvious from the data: a road is drawn between two
    sessions, and a session only exists if it has a live record in
    ~/.claude/sessions. A transcript on its own is never a session, so a
    crossing transcript whose far end has no record produces a road with
    nothing at either end and pkg/city drops it. Both ends have to be
    live or there is no point copying either.

    Until this rule existed the sample home had no roads at all: 27% of
    its absolute file touches went into .claude/, which is not a project
    root, and the Traffic view came out with a grid nobody had painted.
    The live machine has them in quantity, so this was a fixture gap
    rather than a feature gap — but it was a gap that hid one.
    """
    live = [p for p in by_mtime if p.stem in live_ids]
    cwds = {p: session_cwd(p) for p in live}
    roots = {c for c in cwds.values() if c}
    best = None
    for src in live:
        own = cwds[src]
        if not own:
            continue
        for target, count in cross_repo_touches(src, roots, own).items():
            far = next((p for p in live if cwds[p] == target), None)
            if far is not None and far is not src and (best is None or count > best[0]):
                best = (count, src, far)
    if best is None:
        return
    _, src, far = best
    chosen.setdefault(src.stem, src)
    chosen.setdefault(far.stem, far)





def cwd_of(transcript):
    """The cwd a transcript records, which is also what names its project directory."""
    try:
        with open(transcript, encoding="utf-8", errors="replace") as f:
            for line in f:
                line = line.strip().lstrip("\x00")
                if not line:
                    continue
                try:
                    rec = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if isinstance(rec, dict) and isinstance(rec.get("cwd"), str) and rec["cwd"]:
                    return rec["cwd"]
    except OSError:
        pass
    return None


def mapped_rel(src):
    """Where a transcript goes in the fixture, with its project directory renamed.

    ~/.claude/projects/<cwd with / turned to -> is the layout, so the directory
    name carries the path just as plainly as the cwd field does, and scrubbing the
    records while leaving the directory alone renames nothing. It has to be derived
    from the mapped cwd rather than by splitting the directory name on dashes,
    because that split cannot tell the separator in bot-crossing from a path one.
    """
    rel = src.relative_to(HOME)
    cwd = cwd_of(src) if src.suffix == ".jsonl" else None
    if cwd is None:
        parent = src.parent
        while parent != HOME and parent.parent.name != "projects":
            if parent == parent.parent:
                return rel
            parent = parent.parent
        cwd = cwd_of(next(iter(sorted(parent.glob("*.jsonl"))), src))
        if cwd is None:
            return rel
    encoded = map_path(cwd).replace("/", "-")
    parts = list(rel.parts)
    for i, part in enumerate(parts):
        if i and parts[i - 1] == "projects":
            parts[i] = encoded
            break
    return Path(*parts)


def copy_session(src, home_out, raw_out):
    rel = mapped_rel(src)
    scrub_jsonl(src, home_out / rel)
    if raw_out:
        (raw_out / rel).parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(src, raw_out / rel)
    sub = src.parent / src.stem / "subagents"
    count = 0
    if sub.is_dir():
        for f in sorted(sub.iterdir()):
            # A session's subagents live under its own project directory, so
            # they follow the main transcript there. Mapped one by one, a
            # subagent's transcript went by its own first cwd and its
            # .meta.json by whichever transcript sorted first in the folder,
            # and when those differed the pair landed in two projects.
            frel = rel.parent / f.relative_to(src.parent)
            if f.suffix == ".jsonl":
                scrub_jsonl(f, home_out / frel)
            elif f.name.endswith(".meta.json"):
                scrub_json_file(f, home_out / frel)
            else:
                continue
            count += 1
            if raw_out:
                (raw_out / frel).parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(f, raw_out / frel)
    return count


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--name", default="sample", help="fixture name under testing/helpers/fixtures (default: sample)")
    ap.add_argument("--claude-dir", default=str(HOME / ".claude"), help="source directory (default: ~/.claude)")
    ap.add_argument("--recent", type=int, default=3, help="extra most-recently-touched transcripts to include (default: 3)")
    ap.add_argument("--out", default=str(REPO / "testing" / "helpers" / "fixtures"), help="fixtures root (default: testing/helpers/fixtures)")
    ap.add_argument("--no-raw", action="store_true", help="skip the unscrubbed raw/ copy")
    ap.add_argument("--include-self", action="store_true",
                    help="keep the Claude Code session running this script (excluded by default via $CLAUDE_CODE_SESSION_ID)")
    args = ap.parse_args(argv)
    self_id = None if args.include_self else os.environ.get("CLAUDE_CODE_SESSION_ID")

    claude_dir = Path(args.claude_dir).expanduser()
    if not claude_dir.is_dir():
        print(f"{claude_dir} does not exist; nothing to generate", file=sys.stderr)
        return 0
    fixture = Path(args.out) / args.name
    home_out = fixture / "home"
    raw_out = None if args.no_raw else fixture / "raw"
    if fixture.exists():
        shutil.rmtree(fixture)
    home_out.mkdir(parents=True)

    live = {}
    for f in sorted((claude_dir / "sessions").glob("*.json")):
        with open(f, encoding="utf-8") as fh:
            rec = json.load(fh)
        if rec.get("sessionId") == self_id:
            continue
        live[rec.get("sessionId")] = f
        scrub_json_file(f, home_out / f.relative_to(HOME), scrub_session_record)
        if raw_out:
            (raw_out / f.relative_to(HOME)).parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(f, raw_out / f.relative_to(HOME))

    chosen = select_transcripts(claude_dir, set(live), args.recent, self_id)
    sessions = []
    for sid, path in sorted(chosen.items()):
        n = copy_session(path, home_out, raw_out)
        sessions.append({
            "sessionId": sid,
            "project": mapped_rel(path).parent.name,
            "live": sid in live,
            "bridgeStub": is_bridge_stub(path),
            "subagentFiles": n,
            "bytes": path.stat().st_size,
        })

    stats = claude_dir / "stats-cache.json"
    if stats.exists():
        scrub_json_file(stats, home_out / ".claude" / "stats-cache.json", lambda d: d)
        if raw_out:
            shutil.copy2(stats, raw_out / ".claude" / "stats-cache.json")

    claude_json = HOME / ".claude.json"
    if claude_json.exists():
        scrub_json_file(claude_json, home_out / ".claude.json", scrub_claude_json)

    manifest = {
        "args": {"recent": args.recent, "claudeDir": map_path(str(claude_dir))},
        "liveSessions": len(live),
        "sessions": sessions,
    }
    with open(fixture / "manifest.json", "w", encoding="utf-8") as f:
        json.dump(manifest, f, indent=2)
        f.write("\n")

    for leaked in home_out.rglob("*.key"):
        print(f"refusing to keep key file in fixture: {leaked}", file=sys.stderr)
        return 1
    print(f"wrote {fixture}: {len(live)} live records, {len(sessions)} transcripts")
    return 0


if __name__ == "__main__":
    sys.exit(main())
