GitHub PKI
==========

[![Docker Pulls](https://img.shields.io/docker/pulls/camptocamp/github_pki.svg)](https://hub.docker.com/r/camptocamp/github_pki/)
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


