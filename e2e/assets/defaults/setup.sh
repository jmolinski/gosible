#!/usr/bin/bash
# This file is used to set up the environment for the e2e tests.
# It is run once for both Gosible and Ansible.
ssh-keyscan -H managed >> /root/.ssh/known_hosts
