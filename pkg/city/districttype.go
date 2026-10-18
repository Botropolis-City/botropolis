package city

import (
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
)

// DistrictType is what kind of buildings a district grows; a building's
// kind follows its storeys within it (item 109).
type DistrictType string

const (
	ResidentialDistrict DistrictType = "residential"
	ShoppingDistrict    DistrictType = "shopping"
	OfficeDistrict      DistrictType = "office"
	IndustrialDistrict  DistrictType = "industrial"
)

// DistrictTypes are the four, in the order the city offers them.
var DistrictTypes = []DistrictType{ResidentialDistrict, ShoppingDistrict, OfficeDistrict, IndustrialDistrict}

// NextDistrictType is the type after t, coming back round to the first.
func NextDistrictType(t DistrictType) DistrictType {
	for i, d := range DistrictTypes {
		if d == t {
			return DistrictTypes[(i+1)%len(DistrictTypes)]
		}
	}
	return DistrictTypes[0]
}

func parseDistrictType(v string) (DistrictType, bool) {
	for _, d := range DistrictTypes {
		if string(d) == v {
			return d, true
		}
	}
	return "", false
}

// The ways a new district's type can be chosen: hashed from its path,
// guessed from the names in its directory, or one type for all.
const (
	TypingRandom = "random"
	TypingGuess  = "guess"
)

// TypingModes are the district_type setting's values, in the order the
// settings panel offers them.
var TypingModes = []string{TypingRandom, TypingGuess, string(ResidentialDistrict), string(ShoppingDistrict), string(OfficeDistrict), string(IndustrialDistrict)}

// Typing chooses a type for a district that has none yet.
type Typing struct {
	mode  string
	fixed DistrictType
}

// ParseTyping reads the district_type setting; an unknown value is
// random, and reports false so the caller can warn. Empty is random.
func ParseTyping(v string) (Typing, bool) {
	switch v {
	case "", TypingRandom:
		return Typing{mode: TypingRandom}, true
	case TypingGuess:
		return Typing{mode: TypingGuess}, true
	}
	if d, ok := parseDistrictType(v); ok {
		return Typing{mode: string(d), fixed: d}, true
	}
	return Typing{mode: TypingRandom}, false
}

// String is the setting's value for this typing.
func (t Typing) String() string {
	if t.mode == "" {
		return TypingRandom
	}
	return t.mode
}

// Choose is the type a new district at root gets.
func (t Typing) Choose(root string) DistrictType {
	switch {
	case t.fixed != "":
		return t.fixed
	case t.mode == TypingGuess:
		return guessType(root)
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(root))
	return DistrictTypes[h.Sum32()%uint32(len(DistrictTypes))]
}

// What each type's directory tends to hold at its top level, by name
// and by extension. Names are matched lower-cased; nothing is opened.
var (
	industrialNames = map[string]bool{
		"dockerfile": true, "containerfile": true, "compose.yaml": true, "compose.yml": true,
		"docker-compose.yaml": true, "docker-compose.yml": true, "pkgbuild": true, "nfpm.yaml": true,
		"ansible.cfg": true, "playbook.yml": true, "playbook.yaml": true, "kustomization.yaml": true,
		"chart.yaml": true, "helmfile.yaml": true, "vagrantfile": true, "terraform": true, "ansible": true,
		"k8s": true, "kubernetes": true, "helm": true, "charts": true, "infra": true, "packaging": true,
		"roles": true, "manifests": true,
	}
	industrialExts = map[string]bool{".tf": true, ".tfvars": true, ".hcl": true}
	shoppingNames  = map[string]bool{
		"index.html": true, "public": true, "static": true, "app.json": true, "android": true, "ios": true,
		"angular.json": true, "next.config.js": true, "next.config.ts": true, "next.config.mjs": true,
		"vite.config.js": true, "vite.config.ts": true, "svelte.config.js": true, "nuxt.config.ts": true,
		"nuxt.config.js": true, "pubspec.yaml": true,
	}
	shoppingExts = map[string]bool{".html": true, ".css": true, ".xcodeproj": true}
	officeExts   = map[string]bool{
		".md": true, ".txt": true, ".doc": true, ".docx": true, ".pdf": true, ".odt": true, ".rtf": true,
		".csv": true, ".xls": true, ".xlsx": true, ".ods": true, ".ppt": true, ".pptx": true, ".odp": true,
		".key": true, ".pages": true, ".numbers": true, ".tex": true,
	}
	codeNames = map[string]bool{
		"go.mod": true, "package.json": true, "cargo.toml": true, "pyproject.toml": true, "setup.py": true,
		"makefile": true, "gemfile": true, "pom.xml": true, "build.gradle": true, "mix.exs": true,
		"stack.yaml": true, "dune-project": true, "cmakelists.txt": true, "src": true,
	}
	codeExts = map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".rs": true, ".java": true, ".kt": true,
		".c": true, ".h": true, ".cpp": true, ".cs": true, ".rb": true, ".php": true, ".swift": true,
		".hs": true, ".ml": true, ".ex": true, ".scala": true, ".sh": true, ".lua": true,
	}
)

// guessType guesses a district's type from the names at the top of its
// directory: one listing, and no file is opened. Residential when
// nothing points elsewhere, or when the directory cannot be listed.
func guessType(root string) DistrictType {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ResidentialDistrict
	}
	var industrial, shopping, office int
	code := false
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		ext := filepath.Ext(name)
		switch {
		case industrialNames[name] || industrialExts[ext]:
			industrial++
		case shoppingNames[name] || shoppingExts[ext]:
			shopping++
		case officeExts[ext]:
			office++
		}
		if codeNames[name] || codeExts[ext] {
			code = true
		}
	}
	if code {
		office = 0
	}
	best, score := ResidentialDistrict, 0
	for _, c := range []struct {
		t DistrictType
		n int
	}{{IndustrialDistrict, industrial}, {ShoppingDistrict, shopping}, {OfficeDistrict, office}} {
		if c.n > score {
			best, score = c.t, c.n
		}
	}
	return best
}
