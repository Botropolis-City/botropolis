package city_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func directory(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		path := filepath.Join(dir, name)
		if name[len(name)-1] == '/' {
			require.NoError(t, os.MkdirAll(path, 0o755))
			continue
		}
		require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))
	}
	return dir
}

// Item 109: a new district's type, by the district_type setting.
func TestChooseDistrictType(t *testing.T) {
	t.Run("when the mode is random", func(t *testing.T) {
		typing, _ := city.ParseTyping("random")

		t.Run("it should give a district the same type every time", func(t *testing.T) {
			assert.Equal(t, typing.Choose("/w/cinders"), typing.Choose("/w/cinders"))
		})

		t.Run("it should spread twenty districts across more than one type", func(t *testing.T) {
			seen := map[city.DistrictType]bool{}
			for i := range 20 {
				seen[typing.Choose(fmt.Sprintf("/w/project-%d", i))] = true
			}
			assert.Greater(t, len(seen), 1)
		})
	})

	t.Run("when the mode is one type", func(t *testing.T) {
		typing, _ := city.ParseTyping("office")

		t.Run("it should give every new district that type", func(t *testing.T) {
			assert.Equal(t, []city.DistrictType{city.OfficeDistrict, city.OfficeDistrict}, []city.DistrictType{typing.Choose("/w/a"), typing.Choose("/w/b")})
		})
	})

	t.Run("when the mode is guess", func(t *testing.T) {
		typing, _ := city.ParseTyping("guess")
		cases := []struct {
			holds string
			names []string
			want  city.DistrictType
		}{
			{"Terraform files", []string{"main.tf", "variables.tf", "README.md"}, city.IndustrialDistrict},
			{"a Dockerfile and compose file", []string{"Dockerfile", "compose.yaml", "README.md"}, city.IndustrialDistrict},
			{"index.html and public/", []string{"index.html", "public/", "README.md"}, city.ShoppingDistrict},
			{"mostly .md, .pdf and .docx", []string{"notes.md", "plan.md", "report.pdf", "letter.docx"}, city.OfficeDistrict},
			{"a Go library", []string{"go.mod", "lib.go", "lib_test.go", "README.md", "LICENSE"}, city.ResidentialDistrict},
		}
		for _, c := range cases {
			t.Run("and the directory holds "+c.holds, func(t *testing.T) {
				dir := directory(t, c.names...)

				t.Run("it should be "+string(c.want), func(t *testing.T) {
					assert.Equal(t, c.want, typing.Choose(dir))
				})
			})
		}

		t.Run("and the directory is gone", func(t *testing.T) {
			t.Run("it should be residential", func(t *testing.T) {
				assert.Equal(t, city.ResidentialDistrict, typing.Choose(filepath.Join(t.TempDir(), "gone")))
			})
		})

		t.Run("and its files cannot be opened", func(t *testing.T) {
			dir := directory(t, "main.tf", "outputs.tf")
			for _, name := range []string{"main.tf", "outputs.tf"} {
				require.NoError(t, os.Chmod(filepath.Join(dir, name), 0))
			}

			t.Run("it should still guess from their names", func(t *testing.T) {
				assert.Equal(t, city.IndustrialDistrict, typing.Choose(dir))
			})
		})
	})
}

func TestParseTyping(t *testing.T) {
	cases := []struct {
		value string
		mode  string
		ok    bool
	}{
		{"random", "random", true},
		{"guess", "guess", true},
		{"office", "office", true},
		{"", "random", true},
		{"skyscrapers", "random", false},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("when the setting is %q", c.value), func(t *testing.T) {
			typing, ok := city.ParseTyping(c.value)

			t.Run("it should be "+c.mode, func(t *testing.T) {
				assert.Equal(t, [2]any{c.mode, c.ok}, [2]any{typing.String(), ok})
			})
		})
	}
}

func TestLayoutDistrictType(t *testing.T) {
	office, _ := city.ParseTyping("office")
	shopping, _ := city.ParseTyping("shopping")

	t.Run("when a district first appears", func(t *testing.T) {
		l := city.NewLayout()
		l.SetTyping(office)

		t.Run("it should keep the type it was given", func(t *testing.T) {
			assert.Equal(t, city.OfficeDistrict, l.DistrictType("/w/a"))
		})
	})

	t.Run("when the mode changes afterwards", func(t *testing.T) {
		l := city.NewLayout()
		l.SetTyping(office)
		l.DistrictType("/w/a")
		l.SetTyping(shopping)

		t.Run("it should keep the district's type", func(t *testing.T) {
			assert.Equal(t, city.OfficeDistrict, l.DistrictType("/w/a"))
		})
	})

	t.Run("when the layout is saved and loaded again", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "layout.json")
		l := city.NewLayout()
		l.SetTyping(office)
		l.DistrictType("/w/a")
		require.NoError(t, l.Save(path))
		loaded, err := city.LoadLayout(path)
		require.NoError(t, err)
		loaded.SetTyping(shopping)

		t.Run("it should keep the type", func(t *testing.T) {
			assert.Equal(t, city.OfficeDistrict, loaded.DistrictType("/w/a"))
		})
	})

	t.Run("when the type is set by hand", func(t *testing.T) {
		l := city.NewLayout()
		l.SetTyping(office)
		l.DistrictType("/w/a")
		l.SetDistrictType("/w/a", city.IndustrialDistrict)

		t.Run("it should keep that type", func(t *testing.T) {
			assert.Equal(t, city.IndustrialDistrict, l.DistrictType("/w/a"))
		})
	})
}

func TestBuildDistrictTypes(t *testing.T) {
	t.Run("when the city is built", func(t *testing.T) {
		l := city.NewLayout()
		office, _ := city.ParseTyping("office")
		l.SetTyping(office)
		c := city.Build(snapshot(session("a", cinders, state.Working), session("b", mullet, state.Resting)), l)

		t.Run("it should give every building its district's type", func(t *testing.T) {
			var types []city.DistrictType
			for _, b := range c.Buildings() {
				types = append(types, b.Type)
			}
			assert.Equal(t, []city.DistrictType{city.OfficeDistrict, city.OfficeDistrict}, types)
		})
	})
}

func TestDistrictTypeAction(t *testing.T) {
	selected := func(t *testing.T) *city.Scene {
		t.Helper()
		s := scene(t, session("a", cinders, state.Working))
		require.NotNil(t, s.JumpTo(state.Working))
		return s
	}
	typeOf := func(s *city.Scene) city.DistrictType {
		return s.City().Buildings()[0].Type
	}

	t.Run("when a building is selected", func(t *testing.T) {
		t.Run("it should offer to change the district type", func(t *testing.T) {
			assert.Contains(t, selected(t).Actions(), city.ActionDistrictType)
		})
	})

	t.Run("when the action is taken", func(t *testing.T) {
		s := selected(t)
		before := typeOf(s)
		_, note := s.Act(city.ActionDistrictType)
		after := typeOf(s)

		t.Run("it should give the district the next type", func(t *testing.T) {
			assert.Equal(t, city.NextDistrictType(before), after)
		})

		t.Run("it should name the new type in the status", func(t *testing.T) {
			assert.Equal(t, "cinders is now "+string(after), note)
		})

		t.Run("and taken four times", func(t *testing.T) {
			for range 3 {
				s.Act(city.ActionDistrictType)
			}

			t.Run("it should come back to the first", func(t *testing.T) {
				assert.Equal(t, before, typeOf(s))
			})
		})
	})
}
