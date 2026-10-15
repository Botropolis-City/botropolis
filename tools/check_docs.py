import glob, os, re, sys

# The roadmap and its checks (status lines, the open table, the numbering
# that guarantees no item file goes missing) moved to the Botropolis-City
# meta repository at r410, with the history of how five versions of this
# script could not fail. What is left here guards the documents that stayed.

fail = []

def note(check, msg):
    fail.append('%s: %s' % (check, msg))

INDEXES = ['DESIGN.md', 'CLAUDE.md', 'README.md']
docs = INDEXES + ['CONTRIBUTING.md', 'CHANGELOG.md', 'CODE_OF_CONDUCT.md', 'SECURITY.md']
docs += sorted(glob.glob('design/*.md')) + sorted(glob.glob('rules/*.md'))

def targets(p):
    base = os.path.dirname(p) or '.'
    for m in re.finditer(r'\]\(([^)]+)\)', open(p, encoding='utf-8').read()):
        t = m.group(1).split('#')[0]
        if not t or t.startswith(('http', 'mailto')):
            continue
        yield os.path.normpath(os.path.join(base, t))

# 1. links resolve from the linking file's own directory
links = 0
for p in docs:
    for tgt in targets(p):
        links += 1
        if not os.path.exists(tgt):
            note('links', '%s -> %s' % (p, tgt))

# 2. no stale section citations to the pre-split ROADMAP
for p in docs:
    for i, line in enumerate(open(p, encoding='utf-8'), 1):
        for m in re.finditer(r'§\d+', line):
            note('citations', '%s:%d %s refers to a section that no longer exists'
                 % (p, i, m.group(0)))

# 3. the corpus is not empty. Checks 1 and 4 both pass on nothing: an empty
# tree reports "0 links resolve, nothing orphaned" and exits 0. The indexes
# must have content, and DESIGN.md's index links into design/ and rules/, so
# deleting either directory fails check 1 rather than passing on less.
for index in INDEXES:
    if not os.path.exists(index) or not open(index, encoding='utf-8').read().strip():
        note('corpus', '%s is missing or empty' % index)
for d in ('design', 'rules'):
    if not glob.glob(d + '/*.md'):
        note('corpus', '%s/ has no documents' % d)

# 4. nothing is orphaned. Every check above runs in one direction -- it asks
# whether a link points at a file that exists -- and none asks whether a file
# is pointed at, so a document could sit in the tree unreachable by
# navigation with everything green. Everything in design/ and rules/ has to
# be linked from somewhere.
linked = set()
for p in docs:
    linked.update(targets(p))
for p in sorted(glob.glob('design/*.md') + glob.glob('rules/*.md')):
    if os.path.normpath(p) not in linked:
        note('orphans', '%s is linked from nowhere' % p)

if fail:
    for f in fail:
        print('  %s' % f)
    print('\n%d problem(s).' % len(fail))
    sys.exit(1)
print('  %d links resolve, %d documents checked, nothing orphaned. The roadmap\'s checks run in the meta repository.'
      % (links, len(docs)))
