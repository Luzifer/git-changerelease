#!/bin/bash

test_description="Basic binary tests"

. ./sharness.sh

test_expect_success "Ensure CLI is executable" "
  git-changerelease --version
"

test_expect_success "Ensure CLI is loaded as git subcommand" "
  git changerelease --version
"

test_expect_success "Ensure version is replaced" "
  git changerelease --version | grep 'git-changerelease v[0-9]*\.[0-9]*\.[0-9]*'
"

test_expect_success "The tool should be able to write an example config" "
  git changerelease --create-config &&
  test -f ~/.git_changerelease.yaml
"

test_expect_success "The default value for signed tags is expected to be enabled" "
  grep 'disable_signed_tags: false' ~/.git_changerelease.yaml
"

test_done

# vim: set ft=sh :
