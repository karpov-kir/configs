// Package repotest is the in-memory repository the suites drive instead of forking git.
//
// It answers from a table. It does NOT model git: there is no revision grammar here, no index, no
// merge. A case says what git would answer and the code under test is driven against that, which is
// the only way a fake can disagree with the production code rather than agree with itself
// (`ai/kk-flavor/standards/testing.md` rule 5). What could disagree with real git is the one adapter,
// `repo.Exec`, and `repo/exec_test.go` drives that against a real repository.
//
// A path is repository-relative, exactly as git prints one. Content is held per revision, with the
// empty revision meaning the working tree.
package repotest

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"

	"kk-flavor/tools/repo"
)

// WorkTree is the revision name for what is on disk rather than in a commit. Spelt as the empty
// string because that is what repo.Git's callers pass for it.
const WorkTree = ""

// Fake answers repo.Git from what a case put in it. The zero value is an empty repository at "/repo"
// with no commit; New gives it a root and the usual answers.
type Fake struct {
	// Root is what TopLevel answers, and the directory every other answer is anchored to.
	Root string
	// Git is what GitDir answers, and CommonDir too unless Common is set.
	Git string
	// Common is the store linked worktrees share; empty means Git.
	Common string
	// Prefix is what Prefix answers for Root itself. A case asking from a subdirectory sets
	// PrefixByDir instead.
	PrefixByDir map[string]string

	// Revs maps a revision name to the files it holds. Revs[WorkTree] is the working tree.
	Revs map[string]map[string]string
	// Refs resolves a revision name to an object id. A name absent here resolves to nothing, which is
	// what `rev-parse --verify --quiet` answers for an unborn HEAD.
	Refs map[string]string
	// Bases answers MergeBase, keyed "left\x00right" and consulted in both orders.
	Bases map[string]string

	// UntrackedPaths are present in the working tree and not in the index.
	UntrackedPaths []string
	// IgnoredPaths are what git would ignore, and Sources says which rule said so.
	IgnoredPaths []string
	Sources      map[string]string
	// Changes answers ChangedWithStatus for one spelling of the revisions, verbatim, where a case's
	// subject is something the derived comparison below cannot know: a mode, or which blob a side held.
	// The key is the revisions joined by a space.
	Changes map[string][]repo.Change
	// PatchText is what Patch answers, verbatim — the diff a case says git would print. NOT derived
	// from Revs: deriving it would put a diff implementation in this fake, and a suite driven by that
	// would be agreeing with the fake rather than with git.
	PatchText string
	// PatchByRevisions answers Patch for one spelling of the revisions, where a case drives more than
	// one change set. The key is the revisions joined by a space, and PatchText answers anything absent.
	PatchByRevisions map[string]string
	// StatusLines is what `status --porcelain -uall` prints, verbatim.
	StatusLines []string
	// WorktreeList is what `worktree list` prints.
	WorktreeList []repo.Worktree

	// Fail makes the named method return this error, so a case can drive the refusal path without
	// arranging a broken repository. The key is the method name, e.g. "TopLevel".
	Fail map[string]error

	// Added records every path Add staged, in call order, so a case can assert on a write without a
	// repository to inspect.
	Added []string
	// Asked records every method called, in order, for a case whose subject is whether something was
	// asked at all — a memo, or a listing taken once per run.
	Asked []string
}

var _ repo.Git = (*Fake)(nil)

// New is an empty repository rooted at root with one commit, so the common case — a tool that refuses
// where there is no commit — needs no setup to get past. Commit("HEAD", …) fills it.
func New(root string) *Fake {
	return &Fake{
		Root:        root,
		Git:         path.Join(root, ".git"),
		PrefixByDir: map[string]string{},
		Revs:        map[string]map[string]string{WorkTree: {}},
		Refs:        map[string]string{},
		Bases:       map[string]string{},
		Sources:     map[string]string{},
		Fail:        map[string]error{},
	}
}

// Diff sets what Patch answers, for every revision spelling a case does not name on its own.
func (f *Fake) Diff(text string) *Fake {
	f.PatchText = text
	return f
}

// Commit puts files at a revision and gives that revision an object id, so Resolve answers for it.
// The working tree is not touched: a case that wants the same content on disk calls Write too.
func (f *Fake) Commit(rev string, files map[string]string) *Fake {
	if f.Revs == nil {
		f.Revs = map[string]map[string]string{}
	}
	held := map[string]string{}
	for name, body := range files {
		held[name] = body
	}
	f.Revs[rev] = held
	if f.Refs == nil {
		f.Refs = map[string]string{}
	}
	if f.Refs[rev] == "" {
		f.Refs[rev] = objectID(rev)
	}
	return f
}

// Write puts one file in the working tree and the index.
func (f *Fake) Write(name, body string) *Fake {
	if f.Revs == nil {
		f.Revs = map[string]map[string]string{}
	}
	if f.Revs[WorkTree] == nil {
		f.Revs[WorkTree] = map[string]string{}
	}
	f.Revs[WorkTree][name] = body
	return f
}

// AddUntracked puts a file in the working tree without tracking it.
func (f *Fake) AddUntracked(names ...string) *Fake {
	f.UntrackedPaths = append(f.UntrackedPaths, names...)
	return f
}

// Ignore marks paths ignored, with source as what `check-ignore -v` would print for each.
func (f *Fake) Ignore(source string, names ...string) *Fake {
	f.IgnoredPaths = append(f.IgnoredPaths, names...)
	if f.Sources == nil {
		f.Sources = map[string]string{}
	}
	for _, name := range names {
		f.Sources[name] = source
	}
	return f
}

// A stand-in for a git object id: forty hex characters derived from the name, so two revisions differ
// and one revision is stable across calls. Nothing here depends on it being git's own hash.
func objectID(of string) string {
	sum := sha1.Sum([]byte(of))
	return hex.EncodeToString(sum[:])
}

func (f *Fake) note(method string) error {
	f.Asked = append(f.Asked, method)
	return f.Fail[method]
}

func (f *Fake) TopLevel(dir string) (string, error) {
	if err := f.note("TopLevel"); err != nil {
		return "", err
	}
	return f.Root, nil
}

func (f *Fake) CommonDir(dir string) (string, error) {
	if err := f.note("CommonDir"); err != nil {
		return "", err
	}
	if f.Common != "" {
		return f.Common, nil
	}
	return f.Git, nil
}

func (f *Fake) GitDir(dir string) (string, error) {
	if err := f.note("GitDir"); err != nil {
		return "", err
	}
	return f.Git, nil
}

func (f *Fake) GitPath(dir, name string) (string, error) {
	if err := f.note("GitPath"); err != nil {
		return "", err
	}
	return path.Join(f.Git, name), nil
}

func (f *Fake) Prefix(dir string) (string, error) {
	if err := f.note("Prefix"); err != nil {
		return "", err
	}
	return f.PrefixByDir[dir], nil
}

func (f *Fake) Resolve(dir, rev string) (string, error) {
	if err := f.note("Resolve"); err != nil {
		return "", err
	}
	// git peels `^{}` and `^{commit}` off a name it already knows, so a case naming HEAD gets an
	// answer whether or not it spelled the suffix.
	name := rev
	if at := strings.Index(name, "^{"); at >= 0 {
		name = name[:at]
	}
	return f.Refs[name], nil
}

func (f *Fake) MergeBase(dir, left, right string) (string, error) {
	if err := f.note("MergeBase"); err != nil {
		return "", err
	}
	if base, found := f.Bases[left+"\x00"+right]; found {
		return base, nil
	}
	if base, found := f.Bases[right+"\x00"+left]; found {
		return base, nil
	}
	return "", fmt.Errorf("the fake holds no merge base for %s and %s", left, right)
}

func (f *Fake) Tracked(dir string, pathspec ...string) ([]string, error) {
	if err := f.note("Tracked"); err != nil {
		return nil, err
	}
	return underPathspec(sortedKeys(f.Revs[WorkTree]), pathspec), nil
}

func (f *Fake) Untracked(dir string, pathspec ...string) ([]string, error) {
	if err := f.note("Untracked"); err != nil {
		return nil, err
	}
	names := append([]string(nil), f.UntrackedPaths...)
	sort.Strings(names)
	return underPathspec(names, pathspec), nil
}

func (f *Fake) NamesAt(dir, rev string) ([]string, error) {
	if err := f.note("NamesAt"); err != nil {
		return nil, err
	}
	held, known := f.Revs[rev]
	if !known {
		return nil, fmt.Errorf("the fake holds no revision %s", rev)
	}
	return sortedKeys(held), nil
}

// The changed set is what the two sides hold, compared. A case names the revisions it drives with and
// fills them through Commit; the empty revision list compares the working tree against HEAD, which is
// what git does.
func (f *Fake) Changed(dir string, revisions, pathspec []string) ([]string, error) {
	if err := f.note("Changed"); err != nil {
		return nil, err
	}
	changes, err := f.diff(revisions)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, one := range changes {
		if one.Status != "D" {
			names = append(names, one.Path)
		}
	}
	return underPathspec(names, pathspec), nil
}

func (f *Fake) ChangedWithStatus(dir string, revisions, pathspec []string) ([]repo.Change, error) {
	if err := f.note("ChangedWithStatus"); err != nil {
		return nil, err
	}
	changes, found := f.Changes[strings.Join(revisions, " ")]
	if !found {
		var err error
		if changes, err = f.diff(revisions); err != nil {
			return nil, err
		}
	}
	var kept []repo.Change
	for _, one := range changes {
		if len(underPathspec([]string{one.Path}, pathspec)) == 1 {
			kept = append(kept, one)
		}
	}
	return kept, nil
}

// What a derived change calls a file. A case whose subject is a mode — a `.md` that was executable at
// the base, a symlink that became a regular file — states the change itself through Changes rather
// than letting this derive one, because this fake holds content and knows nothing about modes.
const ordinaryFileMode = "100644"

// Two revisions' file maps compared, which is all a diff is to the code under test: a status letter
// per path. `a..b` and `a b` name the same pair, and the empty list means HEAD against the tree.
func (f *Fake) diff(revisions []string) ([]repo.Change, error) {
	left, right := "HEAD", WorkTree
	switch len(revisions) {
	case 0:
	case 1:
		spelt := revisions[0]
		if before, after, closed := strings.Cut(spelt, ".."); closed {
			left, right = strings.TrimSuffix(before, "."), strings.TrimPrefix(after, ".")
			if left == "" {
				left = "HEAD"
			}
			if right == "" {
				right = "HEAD"
			}
		} else {
			left = spelt
		}
	default:
		left, right = revisions[0], revisions[1]
	}
	from, known := f.Revs[left]
	if !known {
		return nil, fmt.Errorf("the fake holds no revision %s to diff from", left)
	}
	to, known := f.Revs[right]
	if !known {
		return nil, fmt.Errorf("the fake holds no revision %s to diff to", right)
	}
	seen := map[string]bool{}
	var changes []repo.Change
	for _, name := range sortedKeys(to) {
		seen[name] = true
		switch was, held := from[name]; {
		case !held:
			changes = append(changes, repo.Change{Status: "A", Path: name,
				NewMode: ordinaryFileMode, Blob: objectID(to[name])})
		case was != to[name]:
			changes = append(changes, repo.Change{Status: "M", Path: name,
				OldMode: ordinaryFileMode, NewMode: ordinaryFileMode,
				OldBlob: objectID(was), Blob: objectID(to[name])})
		}
	}
	for _, name := range sortedKeys(from) {
		if !seen[name] {
			changes = append(changes, repo.Change{Status: "D", Path: name,
				OldMode: ordinaryFileMode, OldBlob: objectID(from[name])})
		}
	}
	return changes, nil
}

func (f *Fake) Patch(dir string, revisions, pathspec []string) ([]byte, error) {
	if err := f.note("Patch"); err != nil {
		return nil, err
	}
	if text, found := f.PatchByRevisions[strings.Join(revisions, " ")]; found {
		return []byte(text), nil
	}
	return []byte(f.PatchText), nil
}

func (f *Fake) Status(dir string) ([]string, error) {
	if err := f.note("Status"); err != nil {
		return nil, err
	}
	return append([]string(nil), f.StatusLines...), nil
}

func (f *Fake) Show(dir, rev, name string) ([]byte, error) {
	if err := f.note("Show"); err != nil {
		return nil, err
	}
	held, known := f.Revs[rev]
	if !known {
		return nil, fmt.Errorf("the fake holds no revision %s", rev)
	}
	body, held2 := held[name]
	if !held2 {
		return nil, fmt.Errorf("%s holds no %s", rev, name)
	}
	return []byte(body), nil
}

// The revision's table, read in the order the paths were asked. A path the revision does not hold is
// passed over and so is one over the cap, which is how a case drives the difference between a file
// that is not there and one the revision holds empty.
func (f *Fake) ContentsAt(dir, rev string, paths []string, maxBytes int64, visit func(string, []byte)) error {
	if err := f.note("ContentsAt"); err != nil {
		return err
	}
	held, known := f.Revs[rev]
	if !known {
		return fmt.Errorf("the fake holds no revision %s", rev)
	}
	for _, name := range paths {
		body, found := held[name]
		if !found || int64(len(body)) > maxBytes {
			continue
		}
		visit(name, []byte(body))
	}
	return nil
}

// Blobs are looked up by the id diff handed out, so a case reads back what it wrote without knowing
// how an id is made.
func (f *Fake) Blob(dir, id string) ([]byte, int64, error) {
	if err := f.note("Blob"); err != nil {
		return nil, 0, err
	}
	for _, held := range f.Revs {
		for _, body := range held {
			if objectID(body) == id {
				return []byte(body), int64(len(body)), nil
			}
		}
	}
	return nil, 0, fmt.Errorf("the fake holds no blob %s", id)
}

func (f *Fake) Ignored(dir string, paths []string) (map[string]bool, error) {
	if err := f.note("Ignored"); err != nil {
		return nil, err
	}
	ignored := map[string]bool{}
	for _, name := range paths {
		for _, rule := range f.IgnoredPaths {
			if name == rule || strings.HasPrefix(name, rule+"/") {
				ignored[name] = true
			}
		}
	}
	return ignored, nil
}

func (f *Fake) IgnoreSource(dir, name string) (string, error) {
	if err := f.note("IgnoreSource"); err != nil {
		return "", err
	}
	return f.Sources[name], nil
}

func (f *Fake) Worktrees(dir string) ([]repo.Worktree, error) {
	if err := f.note("Worktrees"); err != nil {
		return nil, err
	}
	return append([]repo.Worktree(nil), f.WorktreeList...), nil
}

func (f *Fake) Add(dir string, paths []string) error {
	if err := f.note("Add"); err != nil {
		return err
	}
	f.Added = append(f.Added, paths...)
	return nil
}

// git's pathspec, as much of it as these tools use: a path, or a directory every path under it. An
// empty pathspec keeps everything.
func underPathspec(names, pathspec []string) []string {
	if len(pathspec) == 0 {
		return names
	}
	var kept []string
	for _, name := range names {
		for _, spec := range pathspec {
			spec = strings.TrimSuffix(spec, "/")
			if name == spec || spec == "" || spec == "." || strings.HasPrefix(name, spec+"/") {
				kept = append(kept, name)
				break
			}
		}
	}
	return kept
}

func sortedKeys(held map[string]string) []string {
	names := make([]string, 0, len(held))
	for name := range held {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
