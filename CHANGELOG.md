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
