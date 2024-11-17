# e2e testing framework

e2e testing framework tests gosible and ansible's cohesion.
It does so by running same playbook with both gosible and ansible, and comparing filesystems of modified managed nodes. 

## Running e2e tests
By default, e2e will be skipped as they are resource and time-consuming.

To run e2e test use `make e2e-test` command, which will build docker images and run e2e tests.
Use `make e2e-test only=<regex>` to run only tests matching regex.


## Defining test cases
To define test case, create directory with name of test case in `e2e/cases` directory, for example: `e2e/cases/test_name`.
During test execution, all files from created directory will be copied to `e2e/logs/*timestamp*.e2e/test_name/files` directory.
If some files are missing, default files from `e2e/assets/defaults` will be used.
There is no default playbook, so you need to define your own.

Ansible control node will run `run_ansible.sh` script, and Gosible will run `run_gosible.sh` script.

## Test lifecycle
- Run control node and host node containers within the same network.
- Run control node's `run_*.sh` script.
- Commit the image of the modified host node.
- Compare host nodes' images modified by Ansible and Gosible with `container-diff`.

## Results

Logs and diff results are stored in `e2e/logs/*timestamp*.e2e/test_name` directory.

## Internals

__Currently, after each modification of gosible code you need to run `make e2e-build` to rebuild `gosible.dockerfile` image.__
It shouldn't be as bad as it seems as docker cache will be used.

### docker containers
Dockerfiles are located in `e2e/assets/dockerfiles` directory.
- `ansible.dockerfile`: Ansible control node
- `gosible.dockerfile`: Gosible control node
- `diff.dockerfile`: Ubuntu with installed [container-diff](https://github.com/GoogleContainerTools/container-diff) 
- `os/ubuntu.dockerfile`: Ubuntu host node

### ssh
Content of `/e2e/assets/ssh` directory is copied to the control nodes' images, so they can use the private key 
to connect to the host nodes. The public key is copied to the host nodes.

