#!/bin/bash

test_description="Integration test"

. ./sharness.sh

test_expect_success "++ Prepare git repository" "
  git init &&
  git config user.email 'integration@test.ing' &&
  git config user.name 'Sharness'
"

test_expect_success "++ Prepare config file" "
  git changerelease --create-config &&
  sed -i 's/disable_signed_tags: false/disable_signed_tags: true/' ~/.git_changerelease.yaml
"

test_expect_success "Tool should not work on empty repository" "
  test_expect_code 1 git changerelease --no-edit
"

test_expect_success "++ Create first commit" "
  git commit --allow-empty -m 'First commit'
"

test_expect_success "Tool should write changelog with commits available" "
  git changerelease --no-edit
"

test_expect_success "Version is now expected to be 0.1.0" "
  head -n1 History.md | grep '0.1.0'
"

test_expect_success "A tag v0.1.0 should be created" "
  git describe --tag --exact-match | grep v0.1.0
"

test_expect_success "++ Create a fix commit" "
  git commit --allow-empty -m 'fix another empty commit' &&
  git changerelease --no-edit
"

test_expect_success "Version is now expected to be 0.1.1" "
  head -n1 History.md | grep '0.1.1'
"

test_expect_success "A tag v0.1.1 should be created" "
  git describe --tag --exact-match | grep v0.1.1
"

test_expect_success "++ Create commit with non-semver tag" "
  git commit --allow-empty -m 'commit no3' &&
  git tag 'v0.2'
"

test_expect_success "Tool should be able to ignore the non-semver tag" "
  git changerelease --no-edit
"

test_expect_success "Version is now expected to be 0.2.0" "
  head -n1 History.md | grep '0.2.0'
"

test_expect_success "A tag v0.2.0 should be created" "
  git describe --tag --exact-match | grep v0.2.0
"

test_expect_success "++ Create commit breaking change" "
  git commit --allow-empty -m 'breaking: commit no3' &&
  git changerelease --no-edit
"

test_expect_success "Version is now expected to be 1.0.0" "
  head -n1 History.md | grep '1.0.0'
"

test_expect_success "A tag v1.0.0 should be created" "
  git describe --tag --exact-match | grep v1.0.0
"

test_done

# vim: set ft=sh :
