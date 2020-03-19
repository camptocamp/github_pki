GitHub PKI
==========

[![Docker Pulls](https://img.shields.io/docker/pulls/camptocamp/github_pki.svg)](https://hub.docker.com/r/camptocamp/github_pki/)
[![Go Report Card](https://goreportcard.com/badge/github.com/camptocamp/github_pki)](https://goreportcard.com/report/github.com/camptocamp/github_pki)
[![By Camptocamp](https://img.shields.io/badge/by-camptocamp-fb7047.svg)](http://www.camptocamp.com)


github_pki is a command that can be used to retrieve and dump SSH keys from GitHub.


## Examples

### Dump all keys from team `devops` in organization `zeorg` to `/home/bob/.ssh/authorized_keys`

```shell
$ github_pki -a /home/bob/.ssh/authorized_keys \
             -o "zeorg" -T "devops" \
             -t 398d6d326b546d70f9e1ef91abad1fc5ee0f1f39
```

### Dump all keys from specified users as X509 public keys

```shell
$ github_pki -s /etc/software/ssl \
             -u bob -u alice \
             -t 398d6d326b546d70f9e1ef91abad1fc5ee0f1f39
```

### As AuthorizedKeysCommand

github_pki can be used directly in `sshd_config` to be called dynamically
whenever an SSH session is started.

First, make sure `github_pki` is installed in a directory only writable by
root:

```
$ go build
$ sudo cp github_pki /usr/local/bin/
```

Next, configure your `sshd_config`, outputing the keys to stdout:

```
AuthorizedKeysCommand /usr/local/bin/github_pki -t 398d6d326b546d70f9e1ef91abad1fc5ee0f1f39 -u bob -u alice -a -
AuthorizedKeysCommandUser github_pki
```

Make sure the chosen user exists, and use the properly generated GitHub token.

Finally, restarted ssh (e.g. on Ubuntu):

```
$ sudo service ssh restart
```

You should now be able to login to any user on the machine using one of the
GitHub keys specified in the command.


### Using docker

```
$ docker run -v $PWD/authorized_keys:/authorized_keys \
             -e GITHUB_TOKEN=398d6d326b546d70f9e1ef91abad1fc5ee0f1f39 \
             camptocamp/github_pki -u bob -u alice=jessie \
                -a /authorized_keys
```

### Individual user format

Individual users (`-u` or `GITHUB_USERS`) can be passed in the following format:

#### Specify multiple users:

```
$ github_pki -u bob -u alice
```

#### Specify a different name on GitHub

GitHub user `bob` is called `alice` locally:


```
$ github_pki -u bob=alice
```


#### Specify a key ID to use

```
$ github_pki -u bob:1234
```


