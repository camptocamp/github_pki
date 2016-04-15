package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/caarlos0/env"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type user struct {
	Login *string
	Alias *string
}

type environment struct {
  Token          string    `env:"GITHUB_TOKEN"`
  Org            string    `env:"GITHUB_ORG"`
  Teams          []string  `env:"GITHUB_TEAM"`
  Users          []string  `env:"GITHUB_USERS"`
  AuthorizedKeys string    `env:"AUTHORIZED_KEYS"`
  SSLDir         string    `env:"SSL_DIR"`
}

type gitHubPki struct {
	Env    *environment
	Client *github.Client
	Users  []user
	Keys   map[string][]github.Key
}

func main() {
	pki := gitHubPki{}
	pki.getEnv()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: pki.Env.Token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	pki.Client = github.NewClient(tc)

	// Get users from teams
	err := pki.getTeamUsers()
	checkErr(err, "Failed to get team users: %v")

	err = pki.getUsers()
	checkErr(err, "Failed to add individual users: %v")

	err = pki.getUserKeys()
	checkErr(err, "Failed to retrieve user keys: %v")

	err = pki.writeAuthorizedKeys()
	checkErr(err, "Failed to write authorized keys file: %v")

	err = pki.dumpSSLKeys()
	checkErr(err, "Failed to dump SSL keys: %v")
}

func commaSplit(s string) (sl []string) {
	f := func(c rune) bool {
		return c == ','
	}
	sl = strings.FieldsFunc(s, f)
	return
}

func (p *gitHubPki) getEnv() {
	p.Env = &environment{}
  env.Parse(p.Env)
}

func (p *gitHubPki) getTeamUsers() (err error) {
	if p.Env.Org == "" {
		return
	}

	var teams []github.Team

	page := 1
	for page != 0 {
		opt := &github.ListOptions{
			PerPage: 100,
			Page:    page,
		}
		ts, resp, err := p.Client.Organizations.ListTeams(p.Env.Org, opt)
		checkErr(err, "Failed to list teams for organization "+p.Env.Org+": %v")
		page = resp.NextPage
		teams = append(teams, ts...)
	}

	var foundTeams []string

	for _, team := range teams {
		for _, t := range p.Env.Teams {
			if *team.Name == t {
				ghUsers, _, err := p.Client.Organizations.ListTeamMembers(*team.ID, nil)
				checkErr(err, "Failed to list team members for team "+*team.Name+": %v")
				log.Infof("Adding users for team %v", *team.Name)
				for _, ghUser := range ghUsers {
					log.Infof("Adding user %v", *ghUser.Login)
					user := user{ghUser.Login, nil}
					p.addUser(user)
				}
				foundTeams = append(foundTeams, t)
			}
		}

		if len(foundTeams) == len(p.Env.Teams) {
			return
		}
	}

	return
}

func (p *gitHubPki) getUsers() (err error) {
	for _, u := range p.Env.Users {
		user := user{}

		if strings.Contains(u, "=") {
			splitU := strings.Split(u, "=")
			u = splitU[0]
			user.Alias = &splitU[1]
			log.Infof("Adding individual user %v as %v", splitU[0], splitU[1])
		} else {
			log.Infof("Adding individual user %v", u)
		}

		ghUser, _, err := p.Client.Users.Get(u)
		if err != nil {
			log.Errorf("Failed to find user %v", u)
			return err
		}
		user.Login = ghUser.Login
		p.addUser(user)
	}

	return
}

func (p *gitHubPki) addUser(user user) (err error) {
	for _, u := range p.Users {
		if *u.Login == *user.Login {
			if u.Alias == nil && user.Alias == nil {
				log.Infof("Not adding duplicate user %v", *user.Login)
				return
			} else if u.Alias == nil || user.Alias == nil {
				// one of them is set, so we're good
			} else if *u.Alias == *user.Alias {
				log.Infof("Not adding duplicate user %v as %v", *user.Login, *user.Alias)
				return
			}
		}
	}
	p.Users = append(p.Users, user)

	return
}

func (p *gitHubPki) writeAuthorizedKeys() (err error) {
	if p.Env.AuthorizedKeys != "" {
		log.Infof("Generating %v", p.Env.AuthorizedKeys)
		var authorizedKeys []string

		for user, keys := range p.Keys {
			for _, key := range keys {
				authorizedLine := fmt.Sprintf("%v %v_%v", *key.Key, user, *key.ID)
				authorizedKeys = append(authorizedKeys, authorizedLine)
			}
		}

		authorizedBytes := []byte(strings.Join(authorizedKeys, "\n") + "\n")
		err = ioutil.WriteFile(p.Env.AuthorizedKeys, authorizedBytes, 0644)
	}

	return
}

func (p *gitHubPki) dumpSSLKeys() (err error) {
	// And/or dump SSL key
	if p.Env.SSLDir != "" {
		log.Infof("Dumping X509 keys to %v", p.Env.SSLDir)
		os.MkdirAll(p.Env.SSLDir, 0750)

		for user, keys := range p.Keys {
			var sslKeys []string

			for _, key := range keys {
				tmpfile, err := ioutil.TempFile("", "ssh-ssl")
				checkErr(err, "Failed to create tempfile: %v")

				defer os.Remove(tmpfile.Name())
				tmpfile.Write([]byte(*key.Key))

				log.Infof("Converting key %v/%v to X509", user, *key.ID)
				cmd := exec.Command("ssh-keygen", "-f", tmpfile.Name(), "-e", "-m", "pem")

				// TODO: split stdout/stderr in case of errors
				sslKey, err := cmd.CombinedOutput()
				keyStr := fmt.Sprintf("key %v/%v", user, *key.ID)
				if err != nil {
					log.Errorf("Failed to convert "+keyStr+" to X509: %v", err)
				} else {
					sslKeys = append(sslKeys, string(sslKey))
				}
			}

			sslKeyfile := fmt.Sprintf("%s/%v.pem", p.Env.SSLDir, user)

			keys := []byte(strings.Join(sslKeys, "\n") + "\n")
			err = ioutil.WriteFile(sslKeyfile, keys, 0644)
			checkErr(err, "Failed to write key file: %v")
		}
	}

	return
}

func (p *gitHubPki) getUserKeys() (err error) {
	p.Keys = make(map[string][]github.Key)
	for _, user := range p.Users {
		log.Infof("Getting keys for user %v", *user.Login)

		keys, _, err := p.Client.Users.ListKeys(*user.Login, nil)
		checkErr(err, "Failed to list keys for user "+*user.Login)

		var login string
		if user.Alias != nil {
			login = *user.Alias
		} else {
			login = *user.Login
		}

		p.Keys[login] = append(p.Keys[login], keys...)
	}

	return
}

func checkErr(err error, msg string) {
	if err != nil {
		log.Errorf(msg, err)
	}
}
