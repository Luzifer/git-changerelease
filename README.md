![license](https://badges.fyi/github/license/Luzifer/git-changerelease)
![travis](https://badges.fyi/travis/Luzifer/git-changerelease)

# Luzifer / git-changerelease

`git-changerelease` is a git-subcommand to write the changelog in a consistent format and tag it using [semantic versioning](http://semver.org/). You can see the version it writes in the [History.md](History.md) file in this repository.

## Features

- Specify regular expressions to match the commit subject against for automated detection of major / minor / patch releases
- Automatically write Changelog from commits
- Start editor to change the Changelog (and the version) before tagging

## Usage

- Generate a configuration file using `git changerelease --create-config`
- Edit your matchers in the configuration file just created
- Commit and release:

```bash
# git init
Initialized empty Git repository in /tmp/test/.git/

# git commit --allow-empty -m 'add an empty commit'
[master (root-commit) 0cc02e6] add an empty commit

# git-changerelease
# git describe --tags HEAD
v0.1.0

# git commit --allow-empty -m 'fix another empty commit'
[master 69d6f0e] fix another empty commit

# git-changerelease
# git describe --tags HEAD
v0.1.1
```

----

![project status](https://d2o84fseuhwkxk.cloudfront.net/git-changerelease.svg)
