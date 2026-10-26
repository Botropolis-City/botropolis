package city_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
)

func TestTemporaryProjectRoot(t *testing.T) {
	const tmpdir = "/srv/scratch/aria-tmp"
	roots := []struct{ name, root string }{
		{"os.TempDir()", tmpdir},
		{"$TMPDIR", tmpdir},
		{"/tmp", "/tmp"},
		{"/var/tmp", "/var/tmp"},
		{"macOS's /var/folders", "/var/folders/zx/k3j9_x1n2b4c5d6/T"},
		{"macOS's /private/var/folders", "/private/var/folders/zx/k3j9_x1n2b4c5d6/T"},
	}
	for _, r := range roots {
		for _, dir := range []struct{ name, path string }{
			{"the root itself", r.root},
			{"a directory inside it", r.root + "/tmp.Xq81bZ/work"},
		} {
			t.Run("when the directory is "+dir.name+" under "+r.name, func(t *testing.T) {
				t.Setenv("TMPDIR", tmpdir)
				t.Run("it should be the Temporary district", func(t *testing.T) {
					assert.Equal(t, city.TemporaryRoot, city.ProjectRoot(dir.path))
				})
			})
		}
	}

	for _, path := range []string{"/home/u/tmp-project", "/home/u/work/tmp", "/srv/tmpfiles", "/tmpfoo", "/var/tmpfoo/x", "/var/folders-old/x"} {
		t.Run("when the directory "+path+" only contains tmp in its name", func(t *testing.T) {
			t.Setenv("TMPDIR", tmpdir)
			t.Run("it should be its own district", func(t *testing.T) {
				assert.Equal(t, path, city.ProjectRoot(path))
			})
		})
	}
}

func TestTemporaryDistrict(t *testing.T) {
	t.Run("when sessions run in three temporary roots", func(t *testing.T) {
		c := build(t, city.NewLayout(),
			session("a", "/tmp/tmp.Aa11", state.Working),
			session("b", "/var/tmp/run-7", state.Working),
			session("c", "/private/var/folders/zx/k3j9/T/tmp.Bb22", state.Working),
			session("d", cinders, state.Working))

		t.Run("it should make one district for them beside the project's", func(t *testing.T) {
			assert.Len(t, c.Districts, 2)
		})

		t.Run("it should name it Temporary", func(t *testing.T) {
			var names []string
			for _, d := range c.Districts {
				names = append(names, d.Name)
			}
			assert.Contains(t, names, "Temporary")
		})
	})

	t.Run("when a temporary-directory session is tied to a project's district", func(t *testing.T) {
		tied := session("t", "/tmp/tmp.Aa11", state.Working)
		tied.District = cinders + "/.claude/worktrees/glaze"
		c := build(t, city.NewLayout(), session("p", cinders, state.Working), tied, session("u", "/var/tmp/run-7", state.Working))

		t.Run("it should stand in that district", func(t *testing.T) {
			assert.Equal(t, cinders, districtOf(t, c, "t").Root)
		})

		t.Run("and it should leave an untied one to Temporary", func(t *testing.T) {
			assert.Equal(t, city.TemporaryRoot, districtOf(t, c, "u").Root)
		})
	})
}

func districtOf(t *testing.T, c *city.City, id string) *city.District {
	t.Helper()
	for _, d := range c.Districts {
		for _, b := range d.Buildings {
			if b.Session.ID == id {
				return d
			}
		}
	}
	t.Fatalf("no building for %s", id)
	return nil
}
