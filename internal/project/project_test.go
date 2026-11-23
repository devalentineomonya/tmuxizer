package project

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func mkdirs(t *testing.T, root string, rels ...string) {
	t.Helper()
	for _, r := range rels {
		if err := os.MkdirAll(filepath.Join(root, r), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root,
		"group/gitrepo/.git",
		"group/goapp",
		"group/goapp/nested",
		"group/node_modules/dep",
		".hidden/secret",
		"plain/empty",
		"a/b/c/d/toodeep",
	)
	touch(t, filepath.Join(root, "group/goapp/go.mod"))
	touch(t, filepath.Join(root, "group/goapp/nested/go.mod"))
	touch(t, filepath.Join(root, "group/node_modules/dep/package.json"))
	touch(t, filepath.Join(root, ".hidden/secret/package.json"))
	touch(t, filepath.Join(root, "a/b/c/d/toodeep/package.json"))

	var got []string
	for _, p := range Discover([]string{root}, 3, []string{"node_modules"}) {
		rel, _ := filepath.Rel(root, p.Path)
		got = append(got, rel)
	}
	slices.Sort(got)
	want := []string{"group/gitrepo", "group/goapp"}
	if !slices.Equal(got, want) {
		t.Errorf("projects = %v, want %v", got, want)
	}
}

func TestContaining(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "repo/.git", "repo/src/deep")

	p, ok := Containing(filepath.Join(root, "repo/src/deep"))
	if !ok || p.Path != filepath.Join(root, "repo") || p.Name != "repo" {
		t.Errorf("Containing = %+v, %v", p, ok)
	}
	if _, ok := Containing(t.TempDir()); ok {
		t.Error("a bare temp dir is not inside a project")
	}
}
