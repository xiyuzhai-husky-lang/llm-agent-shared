package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"proposal-workflow/internal/workflow"
)

// Config is the project's declared tool and landing contract, loaded once.
type Config struct {
	ProjectKey                string                     `toml:"project_key"`
	ProposalsPath             string                     `toml:"proposals_path"`
	ReviewerTemplate          string                     `toml:"reviewer_template"`
	GuidanceRef               string                     `toml:"guidance_ref"`
	DoubtsLabel               string                     `toml:"doubts_label"`
	MustSignPatterns          string                     `toml:"must_sign_patterns"`
	CodexAddDirs              []string                   `toml:"codex_add_dirs"`
	RoleSandbox               string                     `toml:"role_sandbox"`
	GoSurfaces                string                     `toml:"go_surfaces"`
	Repository                string                     `toml:"-"`
	Main                      string                     `toml:"-"`
	Project                   string                     `toml:"-"`
	State                     string                     `toml:"-"`
	Policies                  map[string]workflow.Policy `toml:"-"`
	TargetBranch              string                     `toml:"target_branch"`
	BranchPrefix              string                     `toml:"branch_prefix"`
	FolderMarker              string                     `toml:"folder_marker"`
	ProjectGate               string                     `toml:"project_gate"`
	ReviewSurfacesGate        string                     `toml:"review_surfaces_gate"`
	LandingDependencies       []string                   `toml:"landing_dependencies"`
	LandingUIDCheck           string                     `toml:"landing_uid_check"`
	LandingMainChanges        string                     `toml:"landing_main_changes"`
	LandingReviewHooks        string                     `toml:"landing_review_hooks"`
	LandingMergeHooks         string                     `toml:"landing_merge_hooks"`
	LandingTrailerEnvironment []string                   `toml:"landing_trailer_environment"`
	LandingTrailerLiterals    []string                   `toml:"landing_trailer_literals"`
	LandingEnvironment        []string                   `toml:"landing_environment"`
	LandingOutcome            string                     `toml:"landing_outcome"`
}

func Load(repo, main, project string) (*Config, error) {
	if !filepath.IsLocal(project) {
		return nil, fmt.Errorf("project must be a repository-relative path")
	}
	repo, err := filepath.Abs(repo)
	if err != nil {
		return nil, err
	}
	main, err = filepath.Abs(main)
	if err != nil {
		return nil, err
	}
	c := &Config{Repository: repo, Main: main, Project: project}
	if err := decode(filepath.Join(repo, project, "proposal-workflow.toml"), c); err != nil {
		return nil, err
	}
	if c.TargetBranch == "" || c.BranchPrefix == "" || c.FolderMarker == "" || c.ReviewSurfacesGate == "" {
		return nil, fmt.Errorf("incomplete proposal-workflow.toml")
	}
	if c.GoSurfaces != "kathy" && c.GoSurfaces != "pdf" {
		return nil, fmt.Errorf("proposal-workflow.toml go_surfaces must select kathy or pdf")
	}
	if c.ProjectKey == "" {
		c.ProjectKey = strings.TrimPrefix(project, "projects/")
	}
	if !filepath.IsLocal(c.ProjectKey) {
		return nil, fmt.Errorf("invalid project_key")
	}
	if c.ProposalsPath == "" {
		c.ProposalsPath = filepath.ToSlash(filepath.Join(project, "proposals"))
	}
	if !filepath.IsLocal(c.ProposalsPath) {
		return nil, fmt.Errorf("proposals_path must be repository-relative")
	}
	if c.RoleSandbox == "" {
		c.RoleSandbox = "workspace-write"
	}
	if c.RoleSandbox != "workspace-write" && c.RoleSandbox != "none" {
		return nil, fmt.Errorf("invalid role_sandbox %q", c.RoleSandbox)
	}
	c.State = filepath.Join(main, ".local", "go-proposal-workflow", project)
	var ranks map[string]Rank
	if err := decode(filepath.Join(repo, "projects/common/orchestrator/mission-ranks.toml"), &ranks); err != nil {
		return nil, err
	}
	c.Policies = make(map[string]workflow.Policy)
	for name, rank := range ranks {
		if rank.Model == "" || rank.Effort == "" || rank.DeadlineMinutes <= 0 || rank.WorkerRevisionCap < 0 {
			return nil, fmt.Errorf("invalid mission rank %s", name)
		}
		c.Policies[name] = workflow.Policy{Rank: name, Model: rank.Model, Effort: rank.Effort, DeadlineSeconds: rank.DeadlineMinutes * 60, WorkerRevisions: rank.WorkerRevisionCap, GateRepairs: 2, WrapRepairs: 2}
	}
	if len(c.Policies) == 0 {
		return nil, fmt.Errorf("mission ranks are empty")
	}
	return c, nil
}

func decode(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err = toml.NewDecoder(strings.NewReader(string(data))).DisallowUnknownFields().Decode(value); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
