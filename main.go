package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/google/go-github/github"
	"github.com/jessevdk/go-flags"
	"golang.org/x/oauth2"
)

type user struct {
	Login *string
	Alias *string
	KeyID *int
}

type config struct {
	Version        bool     `short:"V" long:"version" description:"Display version."`
	Token          string   `short:"t" long:"token" description:"GitHub token" env:"GITHUB_TOKEN"`
	Org            string   `short:"o" long:"org" description:"GitHub organization to include." env:"GITHUB_ORG"`
	Teams          []string `short:"T" long:"teams" description:"GitHub teams to include." env:"GITHUB_TEAM" env-delim:","`
	Users          []string `short:"u" long:"users" description:"GitHub users to include." env:"GITHUB_USERS" env-delim:","`
	AuthorizedKeys string   `short:"a" long:"authorized-keys" description:"authorized_keys file." env:"AUTHORIZED_KEYS"`
	SSLDir         string   `short:"s" long:"ssl-dir" description:"SSL directory to dump X509 keys to." env:"SSL_DIR"`
	Manpage        bool     `short:"m" long:"manpage" description:"Output manpage."`
}

type gitHubPki struct {
	Config *config
	Client *github.Client
	Users  []user
	Keys   map[string][]github.Key
}

var version = "undefined"

func main() {
	pki := gitHubPki{}
	err := pki.getEnv()
	checkErr(err, "Failed to get config: %v")

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: pki.Config.Token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	pki.Client = github.NewClient(tc)

	// First check individual users
	// to enforce specific parameters
	// and avoid duplicates from teams
	err = pki.getUsers()
	checkErr(err, "Failed to add individual users: %v")

	// Get users from teams
	err = pki.getTeamUsers()
	checkErr(err, "Failed to get team users: %v")

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

func (p *gitHubPki) getEnv() (err error) {
	p.Config = &config{}
	parser := flags.NewParser(p.Config, flags.Default)
	if _, err = parser.Parse(); err != nil {
		os.Exit(1)
	}

	if p.Config.Version {
		fmt.Printf("Github_pki v%v\n", version)
		os.Exit(0)
	}

	if p.Config.Manpage {
		var buf bytes.Buffer
		parser.WriteManPage(&buf)
		fmt.Printf(buf.String())
		os.Exit(0)
	}

	return
}

func (p *gitHubPki) getTeamUsers() (err error) {
	if p.Config.Org == "" {
		return
	}

	var teams []*github.Team

	opt := &github.ListOptions{}
	for {
		ts, resp, err := p.Client.Teams.ListTeams(context.Background(), p.Config.Org, opt)
		checkErr(err, "Failed to list teams for organization "+p.Config.Org+": %v")
		teams = append(teams, ts...)
		if opt.Page = resp.NextPage; opt.Page == 0 {
			break
		}
	}

	var foundTeams []string

	for _, team := range teams {
		for _, t := range p.Config.Teams {
			if *team.Name == t {
				var ghUsers []*github.User
				opt := &github.TeamListTeamMembersOptions{
					ListOptions: github.ListOptions{},
				}
				for {
					pageUsers, resp, err := p.Client.Teams.ListTeamMembers(context.Background(), *team.ID, opt)
					checkErr(err, "Failed to list team members for team "+*team.Name+": %v")
					ghUsers = append(ghUsers, pageUsers...)
					if opt.Page = resp.NextPage; opt.Page == 0 {
						break
					}
				}
				log.Infof("Adding users for team %v", *team.Name)
				for _, ghUser := range ghUsers {
					log.Infof("Adding user %v", *ghUser.Login)
					user := user{ghUser.Login, nil, nil}
					p.addUser(user)
				}
				foundTeams = append(foundTeams, t)
			}
		}

		if len(foundTeams) == len(p.Config.Teams) {
			return
		}
	}

	return
}

func (p *gitHubPki) getUsers() (err error) {
	for _, u := range p.Config.Users {
		user := user{}

		if strings.Contains(u, ":") {
			splitU := strings.Split(u, ":")
			u = splitU[0]
			keyID, err := strconv.Atoi(splitU[1])
			if err != nil {
				return err
			}

			user.KeyID = &keyID
			log.Infof("Using key ID %v for user %v", *user.KeyID, u)
		}

		if strings.Contains(u, "=") {
			splitU := strings.Split(u, "=")
			u = splitU[0]
			user.Alias = &splitU[1]
			log.Infof("Adding individual user %v as %v", splitU[0], splitU[1])
		} else {
			log.Infof("Adding individual user %v", u)
		}

		ghUser, _, err := p.Client.Users.Get(context.Background(), u)
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
		if *u.Login ==  *user.Login {
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
	if p.Config.AuthorizedKeys != "" {
		log.Infof("Generating %v", p.Config.AuthorizedKeys)
		var authorizedKeys []string

		for user, keys := range p.Keys {
			for _, key := range keys {
				authorizedLine := fmt.Sprintf("%v %v_%v", *key.Key, user, *key.ID)
				authorizedKeys = append(authorizedKeys, authorizedLine)
			}
		}

		authorizedStr := strings.Join(authorizedKeys, "\n") + "\n"

		if p.Config.AuthorizedKeys == "-" {
			fmt.Print(authorizedStr)
		} else {
			authorizedBytes := []byte(authorizedStr)
			err = ioutil.WriteFile(p.Config.AuthorizedKeys, authorizedBytes, 0644)
		}
	}

	return
}

func (p *gitHubPki) dumpSSLKeys() (err error) {
	// And/or dump SSL key
	if p.Config.SSLDir != "" {
		log.Infof("Dumping X509 keys to %v", p.Config.SSLDir)
		os.MkdirAll(p.Config.SSLDir, 0750)

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

			sslKeyfile := fmt.Sprintf("%s/%v.pem", p.Config.SSLDir, user)

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

		keys, _, err := p.Client.Users.ListKeys(context.Background(), *user.Login, nil)
		checkErr(err, "Failed to list keys for user "+*user.Login)

		var login string
		if user.Alias != nil {
			login = *user.Alias
		} else {
			login = *user.Login
		}

		if user.KeyID != nil {
			for _, k := range keys {
				if int64(*k.ID) == int64(*user.KeyID) {
					p.Keys[login] = append(p.Keys[login], *k)
					break
				}
			}
		} else {
			for _, k := range keys {
				p.Keys[login] = append(p.Keys[login], *k)
			}
		}
	}

	return
}

func checkErr(err error, msg string) {
	if err != nil {
		log.Errorf(msg, err)
	}
}
