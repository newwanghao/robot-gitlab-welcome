package main

import (
	"encoding/base64"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"
)

// SigInfos struct.
type SigInfos struct {
	Name         string       `json:"name,omitempty"`
	Description  string       `json:"description,omitempty"`
	MailingList  string       `json:"mailing_list,omitempty"`
	MeetingURL   string       `json:"meeting_url,omitempty"`
	MatureLevel  string       `json:"mature_level,omitempty"`
	Mentors      []Mentor     `json:"mentors,omitempty"`
	Maintainers  []Maintainer `json:"maintainers,omitempty"`
	Repositories []RepoAdmin  `json:"repositories,omitempty"`
}

// Maintainer struct.
type Maintainer struct {
	GiteeID      string `json:"gitee_id,omitempty"`
	Name         string `json:"name,omitempty"`
	Organization string `json:"organization,omitempty"`
	Email        string `json:"email,omitempty"`
}

// RepoAdmin struct.
type RepoAdmin struct {
	Repo         []string      `json:"repo,omitempty"`
	Admins       []Admin       `json:"admins,omitempty"`
	Committers   []Committer   `json:"committers,omitempty"`
	Contributors []Contributor `json:"contributor,omitempty"`
}

// Contributor struct.
type Contributor struct {
	GiteeID      string `json:"gitee_id,omitempty"`
	Name         string `json:"name,omitempty"`
	Organization string `json:"organization,omitempty"`
	Email        string `json:"email,omitempty"`
}

// Mentor struct.
type Mentor struct {
	GiteeID      string `json:"gitee_id,omitempty"`
	Name         string `json:"name,omitempty"`
	Organization string `json:"organization,omitempty"`
	Email        string `json:"email,omitempty"`
}

// Committer struct.
type Committer struct {
	GiteeID      string `json:"gitee_id,omitempty"`
	Name         string `json:"name,omitempty"`
	Organization string `json:"organization,omitempty"`
	Email        string `json:"email,omitempty"`
}

// Admin struct.
type Admin struct {
	GiteeID      string `json:"gitee_id,omitempty"`
	Name         string `json:"name,omitempty"`
	Organization string `json:"organization,omitempty"`
	Email        string `json:"email,omitempty"`
}

func decodeSigInfoFile(content string) (sets.String, sets.String) {
	maintainers := sets.NewString()

	c, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, nil
	}

	var m SigInfos

	if err = yaml.Unmarshal(c, &m); err != nil {
		return nil, nil
	}

	committers := sets.NewString()

	for _, v := range m.Maintainers {
		maintainers.Insert(v.GiteeID)
	}

	for _, k := range m.Repositories {
		for _, j := range k.Committers {
			committers.Insert(j.GiteeID)
		}
	}

	return maintainers, committers
}

// Relation struct.
type Relation struct {
	Relations []FileOwner `json:"relations" required:"true"`
}

// FileOwner struct.
type FileOwner struct {
	// Path can be a file name or a dir name
	Path  []string     `json:"path" required:"true"`
	Owner []Maintainer `json:"owner,omitempty"`
}
