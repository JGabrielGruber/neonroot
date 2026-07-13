// Package template seeds new workspaces with starter content. Templates come
// from two sources: shipped defaults embedded in the binary, and user templates
// under $XDG_CONFIG_HOME/neonroot/templates/<name>/. User templates win on a
// name clash, so a shipped template can be customized by copying it into config.
//
// Templates are the home for dev-environment ergonomics (editor configs, a
// .tmux.conf, scaffolding): the binary stays a thin engine while the opinionated
// content lives in shareable, editable template directories.
package template

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The `all:` prefix is required so dotfiles like .gitignore are embedded.
//
//go:embed all:files
var files embed.FS

const (
	shippedRoot = "files/templates"
	imageFile   = "files/image/Containerfile"
)

// Source identifies where a template came from.
type Source string

const (
	Shipped Source = "shipped"
	User    Source = "user"
)

// Template describes an available template.
type Template struct {
	Name   string
	Source Source
}

// List returns all templates available for use: shipped defaults plus any user
// templates in userDir. A user template shadows a shipped one of the same name.
func List(userDir string) []Template {
	seen := map[string]Source{}
	// User templates first so they take precedence.
	if entries, err := os.ReadDir(userDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				seen[e.Name()] = User
			}
		}
	}
	for _, name := range shippedNames() {
		if _, ok := seen[name]; !ok {
			seen[name] = Shipped
		}
	}
	out := make([]Template, 0, len(seen))
	for name, src := range seen {
		out = append(out, Template{Name: name, Source: src})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// shippedNames lists the embedded template names.
func shippedNames() []string {
	entries, err := files.ReadDir(shippedRoot)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// Write seeds dstDir from the named template (user dir preferred, else shipped),
// substituting {{workspace}} with wsName. Returns an error if no such template.
func Write(name, userDir, dstDir, wsName string) error {
	if dir := filepath.Join(userDir, name); isDir(dir) {
		return copyTree(os.DirFS(dir), ".", dstDir, wsName)
	}
	sub := filepath.Join(shippedRoot, name)
	if isEmbeddedDir(sub) {
		return copyTree(files, sub, dstDir, wsName)
	}
	return os.ErrNotExist
}

// Scaffold creates a new user template directory (seeded from the shipped
// default) so it can be customized, and returns its path.
func Scaffold(userDir, name string) (string, error) {
	dst := filepath.Join(userDir, name)
	if isDir(dst) {
		return "", os.ErrExist
	}
	if err := copyTree(files, filepath.Join(shippedRoot, "default"), dst, ""); err != nil {
		return "", err
	}
	return dst, nil
}

// WriteImageContainerfile writes the shipped default Containerfile to dst,
// substituting {{image}} with the image name.
func WriteImageContainerfile(dst, name string) error {
	data, err := files.ReadFile(imageFile)
	if err != nil {
		return err
	}
	return writeFile(dst, substitute(data, "{{image}}", name))
}

// copyTree walks src (an fs.FS rooted at root) into dstDir, substituting
// {{workspace}} with wsName in every file.
func copyTree(fsys fs.FS, root, dstDir, wsName string) error {
	return fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		return writeFile(target, substitute(data, "{{workspace}}", wsName))
	})
}

func substitute(data []byte, placeholder, value string) []byte {
	return []byte(strings.ReplaceAll(string(data), placeholder, value))
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isEmbeddedDir(path string) bool {
	info, err := fs.Stat(files, path)
	return err == nil && info.IsDir()
}
