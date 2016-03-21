package main

import (
  "os"
  "os/exec"
  "io/ioutil"
  "strings"
  "fmt"
  "golang.org/x/oauth2"
  "github.com/google/go-github/github"
	"github.com/Sirupsen/logrus"
)

func main() {
  gh_token := os.Getenv("GITHUB_TOKEN")
  ts := oauth2.StaticTokenSource(
    &oauth2.Token{AccessToken: gh_token},
  )
  tc := oauth2.NewClient(oauth2.NoContext, ts)

  client := github.NewClient(tc)

  // Get users from teams
  users, err := getTeamUsers(client)
  if err != nil {
    logrus.Errorf("Failed to get team users")
  }

  users, err = getUsers(client, users)
  if err != nil {
    logrus.Errorf("Failed to add individual users: %v", err)
  }

  keys, err := getUserKeys(client, users)
  if err != nil {
    logrus.Errorf("Failed to retrieve user keys: %v", err)
  }

  err = writeAuthorizedKeys(keys)
  if err != nil {
    logrus.Errorf("Failed to write authorized keys file: %v", err)
  }

  err = dumpSSLKeys(keys)
  if err != nil {
    logrus.Errorf("Failed to dump SSL keys: %v", err)
  }
}

func getTeamUsers(client *github.Client) ([]github.User, error) {
  var users []github.User

  gh_org := os.Getenv("GITHUB_ORG")
  gh_team := os.Getenv("GITHUB_TEAM")

  if gh_org == "" {
    return users, nil
  }

  gh_teams, _, err := client.Organizations.ListTeams(gh_org, nil)
  if err != nil {
    logrus.Errorf("Failed to list teams for organization %v: %v", gh_org, err)
  }

  for _, team := range gh_teams {
    gh_users, _, err := client.Organizations.ListTeamMembers(*team.ID, nil)
    if err != nil {
      logrus.Errorf("Failed to list team members for team %v: %v", *team.ID, err)
    }

    if gh_team == "" || *team.Name == gh_team {
      logrus.Infof("Adding users for team %v", *team.Name)
      for _, user := range gh_users {
        logrus.Infof("Adding user %v", *user.Login)
        users = append(users, user)
      }
    }

    return users, err
  }

  return users, err
}

func getUsers(client *github.Client, users []github.User) ([]github.User, error) {
  var err error

  if os.Getenv("GITHUB_USERS") != "" {
    individualUsers := strings.Split(os.Getenv("GITHUB_USERS"), ",")

    for _, u := range individualUsers {
      logrus.Infof("Adding individual user %v", u)
      user, _, err := client.Users.Get(u)
      if err != nil {
        logrus.Errorf("Failed to find user %v", u)
        return users, err
      }
      users = append(users, *user)
    }
  }

  return users, err
}

func writeAuthorizedKeys(all_keys map[string][]github.Key) (error) {
  var err error

  authorized_file := os.Getenv("AUTHORIZED_KEYS")
  if authorized_file != "" {
    logrus.Infof("Generating %v", authorized_file)
    var authorizedKeys []string

    for user, keys := range all_keys {
      for _, key := range keys {
        authorizedLine := fmt.Sprintf("%v %v_%v", *key.Key, user, *key.ID)
        authorizedKeys = append(authorizedKeys, authorizedLine)
      }
    }

    authorizedBytes := []byte(strings.Join(authorizedKeys, "\n") + "\n")
    ioutil.WriteFile(authorized_file, authorizedBytes, 0644)
  }

  return err
}

func dumpSSLKeys(all_keys map[string][]github.Key) (error) {
  var err error

  // And/or dump SSL key
  ssl_dir := os.Getenv("SSL_DIR")
  if ssl_dir != "" {
    logrus.Infof("Dumping X509 keys to %v", ssl_dir)
    os.MkdirAll(ssl_dir, 0750)

    for user, keys := range all_keys {
      for _, key := range keys {
        tmpfile, err := ioutil.TempFile("", "ssh-ssl")
        if err != nil {
          logrus.Errorf("Failed to create tempfile")
        }
        defer os.Remove(tmpfile.Name())
        tmpfile.Write([]byte(*key.Key))

        logrus.Infof("Converting key %v/%v to X509", user, *key.ID)
        cmd := exec.Command("ssh-keygen", "-f", tmpfile.Name(), "-e", "-m", "pem")

        // TODO: split stdout/stderr in case of errors
        ssl_key, err := cmd.CombinedOutput()
        if err != nil {
          logrus.Errorf("Failed to convert key %v/%v to X509", user, *key.ID)
        }

        ssl_keyfile := fmt.Sprintf("%s/%v_%v.pem", ssl_dir, user, *key.ID)

        err = ioutil.WriteFile(ssl_keyfile, ssl_key, 0644)
        if err != nil {
          logrus.Errorf("Failed to write key to file")
        }
      }
    }
  }

  return err
}


func getUserKeys(client *github.Client, users []github.User) (map[string][]github.Key, error) {
  var err error

  // Store keys in a map of slices
  all_keys := make(map[string][]github.Key)

  for _, user := range users {
    logrus.Infof("Getting keys for user %v", *user.Login)

    keys, _, err := client.Users.ListKeys(*user.Login, nil)
    if err != nil {
      logrus.Errorf("Failed to list keys for user %v", *user.Login)
    }

    for _, k := range keys {
      all_keys[*user.Login] = append(all_keys[*user.Login], k)
    }
  }

  return all_keys, err
}
