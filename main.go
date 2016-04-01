package main

import (
  "os"
  "os/exec"
  "io/ioutil"
  "strings"
  "fmt"
  "golang.org/x/oauth2"
  "github.com/google/go-github/github"
	log "github.com/Sirupsen/logrus"
)

type User struct {
  Login  *string
  Alias  *string
}

type Environment struct {
  Token          string
  Org            string
  Teams          []string
  Users          []string
  AuthorizedKeys string
  SSLDir         string
}

type GitHubPki struct {
  Env         *Environment
  Client      *github.Client
  Users       []User
  Keys        map[string][]github.Key
}

func main() {
  pki := GitHubPki{}
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

func commaSplit(s string) (sl []string, err error) {
  f := func(c rune) bool {
    return c == ','
  }
  sl = strings.FieldsFunc(s, f)
  return
}

func (p *GitHubPki) getEnv() {
  p.Env = &Environment{}
  p.Env.Token = os.Getenv("GITHUB_TOKEN")
  p.Env.Org = os.Getenv("GITHUB_ORG")
  p.Env.Teams, _ = commaSplit(os.Getenv("GITHUB_TEAM"))
  p.Env.Users, _ = commaSplit(os.Getenv("GITHUB_USERS"))
  p.Env.AuthorizedKeys = os.Getenv("AUTHORIZED_KEYS")
}

func (p *GitHubPki) getTeamUsers() (err error) {
  if p.Env.Org == "" {
    return
  }

  var teams []github.Team

  page := 1
  for page != 0 {
    opt := &github.ListOptions{
      PerPage: 100,
      Page: page,
    }
    ts, resp, err := p.Client.Organizations.ListTeams(p.Env.Org, opt)
    checkErr(err, "Failed to list teams for organization "+p.Env.Org+": %v")
    page = resp.NextPage
    for _, t := range ts {
      teams = append(teams, t)
    }
  }

  var found_teams []string

  for _, team := range teams {
    for _, t := range p.Env.Teams {
      if *team.Name == t {
        gh_users, _, err := p.Client.Organizations.ListTeamMembers(*team.ID, nil)
        checkErr(err, "Failed to list team members for team "+*team.Name+": %v")
        log.Infof("Adding users for team %v", *team.Name)
        for _, gh_user := range gh_users {
          log.Infof("Adding user %v", *gh_user.Login)
          user := User{gh_user.Login, nil}
          p.Users = append(p.Users, user)
        }
        found_teams = append(found_teams, t)
      }
    }

    if len(found_teams) == len(p.Env.Teams) {
      return
    }
  }

  return
}

func (p *GitHubPki) getUsers() (err error) {
  for _, u := range p.Env.Users {
    user := User{}

    if strings.Contains(u, "=") {
      split_u := strings.Split(u, "=")
      u = split_u[0]
      user.Alias = &split_u[1]
      log.Infof("Adding individual user %v as %v", split_u[0], split_u[1])
    } else {
      log.Infof("Adding individual user %v", u)
    }

    gh_user, _, err := p.Client.Users.Get(u)
    if err != nil {
      log.Errorf("Failed to find user %v", u)
      return err
    }
    user.Login = gh_user.Login
    p.Users = append(p.Users, user)
  }

  return
}

func (p *GitHubPki) writeAuthorizedKeys() (err error) {
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

func (p *GitHubPki) dumpSSLKeys() (err error) {
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
        ssl_key, err := cmd.CombinedOutput()
        keyStr := fmt.Sprintf("key %v/%v", user, *key.ID)
        if err != nil {
          log.Errorf("Failed to convert "+keyStr+" to X509: %v", err)
        } else {
          sslKeys = append(sslKeys, string(ssl_key))
        }
      }

      ssl_keyfile := fmt.Sprintf("%s/%v.pem", p.Env.SSLDir, user)

      keys := []byte(strings.Join(sslKeys, "\n")+"\n")
      err = ioutil.WriteFile(ssl_keyfile, keys, 0644)
      checkErr(err, "Failed to write key file: %v")
    }
  }

  return
}


func (p *GitHubPki) getUserKeys() (err error) {
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

    for _, k := range keys {
      p.Keys[login] = append(p.Keys[login], k)
    }
  }

  return
}

func checkErr(err error, msg string) {
  if err != nil {
    log.Errorf(msg, err)
  }
}
