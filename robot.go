package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/opensourceways/community-robot-lib/gitlabclient"
	"github.com/opensourceways/community-robot-lib/utils"
	"github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/util/sets"
	"net/http"
	"regexp"
	"sigs.k8s.io/yaml"
	"strings"
)

const (
	botName        = "welcome"
	actionOpen     = "open"
	welcomeMessage = `
Hi ***%s***, welcome to the %s Community.
I'm the Bot here serving you. You can find the instructions on how to interact with me at **[Here](%s)**.
If you have any questions, please contact the SIG: [%s](https://gitee.com/openeuler/community/tree/master/sig/%s), and any of the maintainers: @%s`
	welcomeMessage2 = `
Hi ***%s***, welcome to the %s Community.
I'm the Bot here serving you. You can find the instructions on how to interact with me at **[Here](%s)**.
If you have any questions, please contact the SIG: [%s](https://gitee.com/openeuler/community/tree/master/sig/%s), and any of the maintainers: @%s, any of the committers: @%s`
)

type iClient interface {
	CreateMergeRequestComment(projectID interface{}, mrID int, comment string) error
	AddMergeRequestLabel(projectID interface{}, mrID int, labels gitlab.Labels) error
	GetProjectLabels(projectID interface{}) ([]*gitlab.Label, error)
	CreateProjectLabel(pid interface{}, label, color string) error
	GetDirectoryTree(projectID interface{}, opts gitlab.ListTreeOptions) ([]*gitlab.TreeNode, error)
	ListCollaborators(projectID interface{}) ([]*gitlab.ProjectMember, error)
	CreateIssueComment(projectID interface{}, issueID int, comment string) error
	AddIssueLabels(projectID interface{}, issueID int, labels gitlab.Labels) error
	GetPathContent(projectID interface{}, file, branch string) (*gitlab.File, error)
	GetMergeRequestChanges(projectID interface{}, mrID int) ([]string, error)
	AssignMergeRequest(projectID interface{}, mrID int, ids []int) error
}

func newRobot(cli iClient, gc func() (*configuration, error)) *robot {
	return &robot{getConfig: gc, cli: cli}
}

type robot struct {
	getConfig func() (*configuration, error)
	cli       iClient
}

func (bot *robot) HandleMergeEvent(e *gitlab.MergeEvent, log *logrus.Entry) error {
	if e.ObjectAttributes.Action != actionOpen {
		return nil
	}

	projectID := e.Project.ID
	mrNumber := gitlabclient.GetMRNumber(e)
	author := gitlabclient.GetMRAuthor(e)

	org, repo := gitlabclient.GetMROrgAndRepo(e)
	c, err := bot.getConfig()
	if err != nil {
		return err
	}
	botCfg := c.configFor(org, repo)

	return bot.handle(
		org, repo, author, projectID, botCfg, log,

		func(c string) error {
			return bot.cli.CreateMergeRequestComment(projectID, mrNumber, c)
		},

		func(label string) error {
			return bot.cli.AddMergeRequestLabel(projectID, mrNumber, gitlab.Labels{label})
		},
		mrNumber,
	)
}

func (bot *robot) HandleIssueEvent(e *gitlab.IssueEvent, log *logrus.Entry) error {
	if e.ObjectAttributes.Action != actionOpen {
		return nil
	}
	org, repo := gitlabclient.GetIssueOrgAndRepo(e)
	projectID := e.Project.ID
	number := gitlabclient.GetIssueNumber(e)
	author := gitlabclient.GetIssueAuthor(e)
	c, err := bot.getConfig()
	if err != nil {
		return err
	}
	botCfg := c.configFor(org, repo)

	return bot.handle(
		org, repo, author, projectID, botCfg, log,

		func(c string) error {
			return bot.cli.CreateIssueComment(projectID, number, c)
		},

		func(label string) error {
			return bot.cli.AddIssueLabels(projectID, number, gitlab.Labels{label})
		},
		0,
	)
}

func (bot *robot) handle(
	org, repo, author string,
	projectID int,
	cfg *botConfig, log *logrus.Entry,
	addMsg, addLabel func(string) error,
	number int,
) error {

	mErr := utils.NewMultiErrors()
	if number > 0 {
		resp, err := http.Get(fmt.Sprintf("https://ipb.osinfra.cn/pulls?author=%s", author))
		if err != nil {
			mErr.AddError(err)
		}
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		type T struct {
			Total int `json:"total,omitempty"`
		}

		var t T
		err = json.Unmarshal(body, &t)
		if err != nil {
			mErr.AddError(err)
		}

		if t.Total == 0 {
			if err = bot.cli.AddMergeRequestLabel(projectID, number, []string{"newcomer"}); err != nil {
				mErr.AddError(err)
			}
		}
	}

	sigName, comment, err := bot.genComment(org, repo, author, number, projectID, cfg, log)
	if err != nil {
		return err
	}

	if err := addMsg(comment); err != nil {
		mErr.AddError(err)
	}

	label := fmt.Sprintf("sig/%s", sigName)

	if err := bot.createLabelIfNeed(projectID, label); err != nil {
		log.Errorf("create repo label:%s, err:%s", label, err.Error())
	}

	if err := addLabel(label); err != nil {
		mErr.AddError(err)
	}

	return mErr.Err()
}

func (bot robot) genComment(org, repo, author string, number, pid int, cfg *botConfig, log *logrus.Entry) (string, string, error) {

	sigName, err := bot.getSigOfRepo(org, repo, pid, cfg)
	if err != nil {
		return "", "", err
	}

	if sigName == "" {
		return "", "", fmt.Errorf("cant get sig name of repo: %s/%s", org, repo)
	}

	maintainers, committers, err := bot.getMaintainers(org, repo, sigName, number, pid, cfg, log)
	if err != nil {
		return "", "", err
	}

	if cfg.NeedAssign && number != 0 {
		if err = bot.cli.AssignMergeRequest(pid, number, []int{}); err != nil {
			return "", "", err
		}
	}

	if len(committers) != 0 {
		return sigName, fmt.Sprintf(
			welcomeMessage2, author, cfg.CommunityName, cfg.CommandLink,
			sigName, sigName, strings.Join(maintainers, " , @"), strings.Join(committers, " , @"),
		), nil
	}

	return sigName, fmt.Sprintf(
		welcomeMessage, author, cfg.CommunityName, cfg.CommandLink,
		sigName, sigName, strings.Join(maintainers, " , @"),
	), nil
}

func (bot *robot) getMaintainers(org, repo, sig string, number, pid int, cfg *botConfig, log *logrus.Entry) ([]string, []string, error) {
	if cfg.WelcomeSimpler {
		membersToContact, err := bot.findSpecialContact(org, repo, number, pid, cfg, log)
		if err == nil && len(membersToContact) != 0 {
			return membersToContact.UnsortedList(), nil, nil
		}
	}

	v, err := bot.cli.ListCollaborators(pid)
	if err != nil {
		return nil, nil, err
	}

	r := make([]string, 0, len(v))
	for i := range v {
		p := v[i]
		if p != nil && (p.AccessLevel == 30 || p.AccessLevel == 40 || p.AccessLevel == 50) {
			r = append(r, v[i].Username)
		}
	}

	f, err := bot.cli.GetPathContent(pid, fmt.Sprintf("sig/%s/OWNERS", sig), "master")
	if err != nil || len(f.Content) == 0 {
		return r, nil, err
	}

	s, err := bot.cli.GetPathContent(pid, fmt.Sprintf("sig/%s/sig-info.yaml", sig), "master")
	if err != nil || len(s.Content) == 0 {
		return r, nil, err
	}

	maintainers, committers := decodeSigInfoFile(s.Content)
	return maintainers.UnsortedList(), committers.UnsortedList(), nil
}

func (bot *robot) createLabelIfNeed(pid int, label string) error {
	repoLabels, err := bot.cli.GetProjectLabels(pid)
	if err != nil {
		return err
	}

	for _, v := range repoLabels {
		if v.Name == label {
			return nil
		}
	}

	return bot.cli.CreateProjectLabel(pid, label, "")
}

func (bot *robot) findSpecialContact(org, repo string, number, pid int, cfg *botConfig, log *logrus.Entry) (sets.String, error) {
	if number == 0 {
		return nil, nil
	}

	changes, err := bot.cli.GetMergeRequestChanges(pid, number)
	if err != nil {
		log.Errorf("get pr changes failed: %v", err)
		return nil, err
	}

	filePath := cfg.FilePath
	branch := cfg.FileBranch

	content, err := bot.cli.GetPathContent(pid, filePath, branch)
	if err != nil {
		log.Errorf("get file %s/%s/%s failed, err: %v", org, repo, filePath, err)
		return nil, err
	}

	c, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		log.Errorf("decode string err: %v", err)
		return nil, err
	}

	var r Relation

	err = yaml.Unmarshal(c, &r)
	if err != nil {
		log.Errorf("yaml unmarshal failed: %v", err)
		return nil, err
	}

	owners := sets.NewString()
	var mo []Maintainer
	for _, c := range changes {
		for _, f := range r.Relations {
			for _, ff := range f.Path {
				if strings.Contains(c, ff) {
					mo = append(mo, f.Owner...)
				}
				if strings.Contains(ff, "/*/") {
					reg := regexp.MustCompile(strings.Replace(ff, "/*/", "/[^\\s]+/", -1))
					if ok := reg.MatchString(c); ok {
						mo = append(mo, f.Owner...)
					}
				}
			}
		}
	}

	for _, m := range mo {
		owners.Insert(m.GiteeID)
	}

	return owners, nil
}
