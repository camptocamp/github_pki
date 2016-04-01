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

type GitHubPki struct {
  Client *github.Client
  Users  []User
  Keys   map[string][]github.Key
}

func main() {
  gh_token := os.Getenv("GITHUB_TOKEN")
  ts := oauth2.StaticTokenSource(
    &oauth2.Token{AccessToken: gh_token},
  )
  tc := oauth2.NewClient(oauth2.NoContext, ts)

  pki := GitHubPki{}
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

func (p *GitHubPki) getTeamUsers() (err error) {
  gh_org := os.Getenv("GITHUB_ORG")
  gh_teams := strings.Split(os.Getenv("GITHUB_TEAM"), ",")

  if gh_org == "" {
    return
  }

  var teams []github.Team

  page := 1
  for page != 0 {
    opt := &github.ListOptions{
      PerPage: 100,
      Page: page,
    }
    ts, resp, err := p.Client.Organizations.ListTeams(gh_org, opt)
    checkErr(err, "Failed to list teams for organization "+gh_org+": %v")
    page = resp.NextPage
    for _, t := range ts {
      teams = append(teams, t)
    }
  }

  var found_teams []string

  for _, team := range teams {
    for _, t := range gh_teams {
      if os.Getenv("GITHUB_TEAM") == "" || *team.Name == t {
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

    if len(found_teams) == len(gh_teams) {
      return
    }
  }

  return
}

func (p *GitHubPki) getUsers() (err error) {
  if os.Getenv("GITHUB_USERS") != "" {
    individualUsers := strings.Split(os.Getenv("GITHUB_USERS"), ",")

    for _, u := range individualUsers {
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
  }

  return
}

func (p *GitHubPki) writeAuthorizedKeys() (err error) {
  authorized_file := os.Getenv("AUTHORIZED_KEYS")
  if authorized_file != "" {
    log.Infof("Generating %v", authorized_file)
    var authorizedKeys []string

    for user, keys := range p.Keys {
      for _, key := range keys {
        authorizedLine := fmt.Sprintf("%v %v_%v", *key.Key, user, *key.ID)
        authorizedKeys = append(authorizedKeys, authorizedLine)
      }
    }

    authorizedBytes := []byte(strings.Join(authorizedKeys, "\n") + "\n")
    err = ioutil.WriteFile(authorized_file, authorizedBytes, 0644)
  }

  return
}

func (p *GitHubPki) dumpSSLKeys() (err error) {
  // And/or dump SSL key
  ssl_dir := os.Getenv("SSL_DIR")
  if ssl_dir != "" {
    log.Infof("Dumping X509 keys to %v", ssl_dir)
    os.MkdirAll(ssl_dir, 0750)

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

      ssl_keyfile := fmt.Sprintf("%s/%v.pem", ssl_dir, user)

      keys := []byte(strings.Join(sslKeys, "\n")+"\n")
      err = ioutil.WriteFile(ssl_keyfile, keys, 0644)
      checkErr(err, "Failed to write key file: %v")
    }
  }

  return
}


func (p *GitHubPki) getUserKeys() (err error) {
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
