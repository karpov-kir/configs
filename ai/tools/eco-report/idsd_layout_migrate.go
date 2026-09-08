package ecoreport

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"kk-flavor/tools/shell"
)

type layoutMove struct {
	source, destination string
	decisionBody        []byte
}

func (r *run) cmdMigrateLayout() {
	// Dry-run leaves even the lock file absent. Apply shares the report lock with lifecycle commands.
	if r.arg(2) == "--apply" {
		lock := r.lockReports()
		defer lock.Close()
	}
	moves, err := r.planLayoutMigration()
	if err != nil {
		r.refuse("error: idsd layout migration refused: " + shell.Oneline(err.Error()))
	}
	rewrites, err := r.planLayoutIgnoreMigration()
	if err != nil {
		r.refuse("error: idsd layout migration refused: " + shell.Oneline(err.Error()))
	}
	for _, rewrite := range rewrites {
		r.line("update owned ignore patterns: %s", shell.Oneline(rewrite.path))
	}
	for _, move := range moves {
		if move.source == move.destination {
			r.line("add decision sections: %s", shell.Oneline(move.source))
			continue
		}
		r.line("move %s -> %s", shell.Oneline(move.source), shell.Oneline(move.destination))
	}
	if r.arg(2) == "--dry-run" {
		r.line("dry-run: %d move(s); no files changed", len(moves))
		return
	}
	for _, move := range moves {
		if err := os.MkdirAll(filepath.Dir(move.destination), 0o700); err != nil {
			r.refuse("error: migration stopped before " + shell.Oneline(move.source+": "+err.Error()))
		}
		if err := os.Rename(move.source, move.destination); err != nil {
			r.refuse("error: migration stopped at " + shell.Oneline(move.source+": "+err.Error()) + "; already completed moves are retained")
		}
		if move.decisionBody != nil {
			if err := writeMigratedDecision(move.destination, move.decisionBody); err != nil {
				r.refuse("error: decision record moved intact but section migration failed at " + shell.Oneline(move.destination+": "+err.Error()))
			}
		}
	}
	for _, rewrite := range rewrites {
		if err := writeLayoutIgnore(rewrite); err != nil {
			r.refuse("error: artifacts migrated but ignore update failed: " + shell.Oneline(err.Error()))
		}
	}
	r.line("migrated idsd layout: %d move(s); run report.sh layout check and curate preserved constraints into charter.md", len(moves))
}

func (r *run) planLayoutMigration() ([]layoutMove, error) {
	if _, err := os.Lstat(r.mergeSlotPath()); err == nil {
		return nil, fmt.Errorf("merge slot is held; finish its ship before migration")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if _, err := os.Lstat(r.idsdDir); err != nil {
		return nil, err
	}
	err := filepath.WalkDir(r.idsdDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink at %s; nothing moved", path)
		}
		if entry.Name() == reportName {
			return fmt.Errorf("open report at %s; complete and close it before migration (use the pinned previous runtime for a legacy report)", path)
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported file type at %s", path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var moves []layoutMove
	err = r.planProjectMoves(&moves)
	if err != nil {
		return nil, err
	}
	destinations := map[string]bool{}
	for i := range moves {
		move := &moves[i]
		if destinations[move.destination] {
			return nil, fmt.Errorf("collision at %s", move.destination)
		}
		destinations[move.destination] = true
		if filepath.Base(move.destination) == "decisions.md" && filepath.Base(filepath.Dir(move.destination)) == agentsDirName {
			body, err := os.ReadFile(move.source)
			if err != nil {
				return nil, err
			}
			move.decisionBody, err = legacyDecisionBody(body)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", move.source, err)
			}
		}
		if _, err := os.Lstat(move.destination); err == nil && move.source != move.destination {
			return nil, fmt.Errorf("collision at %s; both copies are preserved", move.destination)
		} else if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		parent := filepath.Dir(move.destination)
		for parent != r.idsdDir {
			if info, err := os.Lstat(parent); err == nil && !info.IsDir() {
				return nil, fmt.Errorf("destination parent is not a directory: %s", parent)
			} else if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			parent = filepath.Dir(parent)
		}
	}
	return moves, nil
}

func (r *run) planProjectMoves(moves *[]layoutMove) error {
	entries, err := os.ReadDir(r.idsdDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		source := filepath.Join(r.idsdDir, entry.Name())
		switch entry.Name() {
		case "charter.md", "roadmap.md":
			if entry.IsDir() {
				return fmt.Errorf("expected a regular human file: %s", source)
			}
		case agentsDirName:
			if err := planAgentExtras(source, moves); err != nil {
				return err
			}
		case "intents", "archive":
			if err := r.planShips(source, moves); err != nil {
				return err
			}
		default:
			destination := filepath.Join(r.projectAgentsDir(), "supporting", entry.Name())
			if slices.Contains(agentRecordFiles, entry.Name()) && !entry.IsDir() {
				destination = filepath.Join(r.projectAgentsDir(), entry.Name())
			}
			*moves = append(*moves, layoutMove{source: source, destination: destination})
		}
	}
	return nil
}

func (r *run) planShips(group string, moves *[]layoutMove) error {
	entries, err := os.ReadDir(group)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		source := filepath.Join(group, entry.Name())
		if !entry.IsDir() || reportNameFor(entry.Name()) != entry.Name() {
			*moves = append(*moves, layoutMove{source: source, destination: filepath.Join(r.projectAgentsDir(), "supporting", filepath.Base(group), entry.Name())})
			continue
		}
		children, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, child := range children {
			path := filepath.Join(source, child.Name())
			switch child.Name() {
			case intentName:
				if child.IsDir() {
					return fmt.Errorf("expected a regular intent file: %s", path)
				}
			case agentsDirName:
				if err := planAgentExtras(path, moves); err != nil {
					return err
				}
			default:
				destination := filepath.Join(source, agentsDirName, "supporting", child.Name())
				if slices.Contains(agentRecordFiles, child.Name()) && !child.IsDir() {
					destination = filepath.Join(source, agentsDirName, child.Name())
				}
				*moves = append(*moves, layoutMove{source: path, destination: destination})
			}
		}
	}
	return nil
}

func planAgentExtras(dir string, moves *[]layoutMove) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "supporting" {
			if !entry.IsDir() {
				return fmt.Errorf("supporting must be a directory: %s", dir)
			}
			continue
		}
		if slices.Contains(agentRecordFiles, entry.Name()) && !entry.IsDir() {
			if entry.Name() == "decisions.md" {
				path := filepath.Join(dir, entry.Name())
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				normalized, err := legacyDecisionBody(body)
				if err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
				if normalized != nil {
					*moves = append(*moves, layoutMove{source: path, destination: path, decisionBody: normalized})
				}
			}
			continue
		}
		*moves = append(*moves, layoutMove{source: filepath.Join(dir, entry.Name()), destination: filepath.Join(dir, "supporting", entry.Name())})
	}
	return nil
}

func legacyDecisionBody(body []byte) ([]byte, error) {
	text := string(body)
	lines := shell.SplitLines(text)
	hasSections := false
	for _, line := range lines {
		if line == "## Promotion candidates" || line == "## Decisions" {
			hasSections = true
		}
	}
	if hasSections {
		return nil, decisionSectionsError(lines)
	}
	normalized := text + decisionHeadings
	for i, line := range lines {
		if _, ok := parseRecordEntry(i, line); ok {
			normalized = strings.Join(lines[:i], "\n") + decisionHeadings + strings.Join(lines[i:], "\n") + "\n"
			break
		}
	}
	if err := decisionSectionsError(shell.SplitLines(normalized)); err != nil {
		return nil, err
	}
	return []byte(normalized), nil
}

func writeMigratedDecision(path string, body []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".layout-decision-")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if _, err = temporary.Write(body); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), path)
}
