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

type User struct {
  Login  *string
  Alias  *string
}

func main() {
  gh_token := os.Getenv("GITHUB_TOKEN")
  ts := oauth2.StaticTokenSource(
    &oauth2.Token{AccessToken: gh_token},
  )
  tc := oauth2.NewClient(oauth2.NoContext, ts)

  client := github.NewClient(tc)

  // Get users from teams
  users, err := getTeamUsers(client)
  checkErr(err, "Failed to get team users: %v")

  users, err = getUsers(client, users)
  checkErr(err, "Failed to add individual users: %v")

  keys, err := getUserKeys(client, users)
  checkErr(err, "Failed to retrieve user keys: %v")

  err = writeAuthorizedKeys(keys)
  checkErr(err, "Failed to write authorized keys file: %v")

  err = dumpSSLKeys(keys)
  checkErr(err, "Failed to dump SSL keys: %v")
}

func getTeamUsers(client *github.Client) ([]User, error) {
  var users []User

  gh_org := os.Getenv("GITHUB_ORG")
  gh_teams := strings.Split(os.Getenv("GITHUB_TEAM"), ",")

  if gh_org == "" {
    return users, nil
  }

  teams, _, err := client.Organizations.ListTeams(gh_org, nil)
  checkErr(err, "Failed to list teams for organization "+gh_org+": %v")

  var found_teams []string

  for _, team := range teams {
    gh_users, _, err := client.Organizations.ListTeamMembers(*team.ID, nil)
    checkErr(err, "Failed to list team members for team "+*team.Name+": %v")

    for _, t := range gh_teams {
      if os.Getenv("GITHUB_TEAM") == "" || *team.Name == t {
        logrus.Infof("Adding users for team %v", *team.Name)
        for _, gh_user := range gh_users {
          logrus.Infof("Adding user %v", *gh_user.Login)
          user := User{gh_user.Login, nil}
          users = append(users, user)
        }
        found_teams = append(found_teams, t)
      }
    }

    if len(found_teams) == len(gh_teams) {
      return users, err
    }
  }

  return users, err
}

func getUsers(client *github.Client, users []User) ([]User, error) {
  var err error

  if os.Getenv("GITHUB_USERS") != "" {
    individualUsers := strings.Split(os.Getenv("GITHUB_USERS"), ",")

    for _, u := range individualUsers {
      user := User{}

      if strings.Contains(u, "=") {
        split_u := strings.Split(u, "=")
        u = split_u[0]
        user.Alias = &split_u[1]
        logrus.Infof("Adding individual user %v as %v", split_u[0], split_u[1])
      } else {
        logrus.Infof("Adding individual user %v", u)
      }

      gh_user, _, err := client.Users.Get(u)
      if err != nil {
        logrus.Errorf("Failed to find user %v", u)
        return users, err
      }
      user.Login = gh_user.Login
      users = append(users, user)
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
    err = ioutil.WriteFile(authorized_file, authorizedBytes, 0644)
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
      var sslKeys []string

      for _, key := range keys {
        tmpfile, err := ioutil.TempFile("", "ssh-ssl")
        checkErr(err, "Failed to create tempfile: %v")

        defer os.Remove(tmpfile.Name())
        tmpfile.Write([]byte(*key.Key))

        logrus.Infof("Converting key %v/%v to X509", user, *key.ID)
        cmd := exec.Command("ssh-keygen", "-f", tmpfile.Name(), "-e", "-m", "pem")

        // TODO: split stdout/stderr in case of errors
        ssl_key, err := cmd.CombinedOutput()
        keyStr := fmt.Sprintf("key %v/%v", user, *key.ID)
        if err != nil {
          logrus.Errorf("Failed to convert "+keyStr+" to X509: %v", err)
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

  return err
}


func getUserKeys(client *github.Client, users []User) (map[string][]github.Key, error) {
  var err error

  // Store keys in a map of slices
  all_keys := make(map[string][]github.Key)

  for _, user := range users {
    logrus.Infof("Getting keys for user %v", *user.Login)

    keys, _, err := client.Users.ListKeys(*user.Login, nil)
    checkErr(err, "Failed to list keys for user "+*user.Login)

    var login string
    if user.Alias != nil {
      login = *user.Alias
    } else {
      login = *user.Login
    }

    for _, k := range keys {
      all_keys[login] = append(all_keys[login], k)
    }
  }

  return all_keys, err
}

func checkErr(err error, msg string) {
  if err != nil {
    logrus.Errorf(msg, err)
  }
}
