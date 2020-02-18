# [0.10.4](https://github.com/camptocamp/github_pki/releases/tag/0.10.4) (2020-02-18)

* Bugfixes:

  - Dockerfile: followup on golang:onbuild depreciation [Marc Fournier]
  - Modify the code for listing teams with pagination [Guewen Baconnier]
  - Bump golang version to 1.11 [Julien]
  - Team methods has moved in Teams object [Julien]
  - fixing build [Pierre Mauduit]

# [0.10.3](https://github.com/camptocamp/github_pki/releases/tag/0.10.3) (2017-02-23)

* Bugfixes:

  - New version of go-github requires context

# [0.10.2](https://github.com/camptocamp/github_pki/releases/tag/0.10.2) (2016-06-21)

* Bugfixes:

  - Process individual users first

# [0.10.1](https://github.com/camptocamp/github_pki/releases/tag/0.10.1) (2016-06-21)

* Bugfixes:

  - Specify env-delim for environment variables feeding slices

# [0.10.0](https://github.com/camptocamp/github_pki/releases/tag/0.10.0) (2016-06-21)

* Docker:

  - Use golang:onbuild again (we need `ssh-keygen`)

# [0.9.0](https://github.com/camptocamp/github_pki/releases/tag/0.9.0) (2016-06-21)

* Features:

  - Add usage and flags
  - Add key filtering for users (fix #2)

* Internals:

  - Get rid of Godeps
  - Use go-flags instead of env

* Continuous Integration:

  - Integrate with Travis CI
  - Add Makefile
  - Build statically

* Docker:

  - Build image from scratch
