FROM willhallonline/ansible:2.12-ubuntu-22.04
# TODO: This Dockerfile comes with mitogen, but I don't know if it is working by default.

COPY e2e/assets/ssh /root/.ssh
RUN chmod -R 600 /root/.ssh

WORKDIR /test_ground

CMD ["sleep", "infinity"]