package repository

import (
	"fmt"
	"os"
	"path/filepath"
)

// prepareSkills installs the existing local integration omitted by Git worktrees.
// It copies no workflow state and never overwrites an operator's installed skills.
func (r Repository) prepareSkills(directory string) error {
	destination := filepath.Join(directory, ".agents", "skills")
	if info, err := os.Stat(destination); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("slot skills path is not a directory: %s", destination)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	source := filepath.Join(r.Config.Main, ".agents", "skills")
	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		return fmt.Errorf("install slot skills from %s: %w", source, err)
	}
	return nil
}
