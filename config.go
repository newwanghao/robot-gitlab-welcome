package main

import (
	"fmt"
	"github.com/opensourceways/community-robot-lib/config"
)

type configuration struct {
	ConfigItems []botConfig `json:"config_items,omitempty"`
}

func (c *configuration) configFor(org, repo string) *botConfig {
	if c == nil {
		return nil
	}

	items := c.ConfigItems
	v := make([]config.IRepoFilter, len(items))
	for i := range items {
		v[i] = &items[i]
	}

	if i := config.Find(org, repo, v); i >= 0 {
		return &items[i]
	}

	return nil
}

func (c *configuration) Validate() error {
	if c == nil {
		return nil
	}

	items := c.ConfigItems
	for i := range items {
		if err := items[i].validate(); err != nil {
			return err
		}
	}

	return nil
}

func (c *configuration) SetDefault() {
	if c == nil {
		return
	}

	Items := c.ConfigItems
	for i := range Items {
		Items[i].setDefault()
	}
}

type botConfig struct {
	config.RepoFilter
	// CommunityName is the name of community
	CommunityName string `json:"community_name" required:"true"`

	// CommandLink is the link to command help document page.
	CommandLink string `json:"command_link" required:"true"`

	// CommunityRepo is used to read file path
	CommunityRepo string `json:"community_repo" required:"true"`

	// Branch is used to read file path
	Branch string `json:"branch" required:"true"`

	// FilePath is the path-owner-map file path
	FilePath string `json:"file_path,omitempty"`

	// FileBranch is used to located FilePath
	FileBranch string `json:"file_branch,omitempty"`

	// NeedAssign decides assign maintainers to PR or not
	NeedAssign bool `json:"need_assign,omitempty"`

	// WelcomeSimpler means to make the welcome message simpler when PR is opened
	WelcomeSimpler bool `json:"welcome_simpler,omitempty"`

	// Path is used to read file path
	Path string `json:"path" required:"true"`

	// reposSig is used to cache information
	reposSig map[string]string
}

func (c *botConfig) setDefault() {
}

func (c *botConfig) validate() error {
	if c.CommunityName == "" {
		return fmt.Errorf("the community_name configuration can not be empty")
	}

	if c.CommandLink == "" {
		return fmt.Errorf("the command_link configuration can not be empty")
	}

	if c.CommunityRepo == "" {
		return fmt.Errorf("the community_repo configuration can not be empty")
	}

	if c.Branch == "" {
		return fmt.Errorf("the branch configuration can not be empty")
	}

	return c.RepoFilter.Validate()
}
