FROM ubuntu:22.04

ENV TZ=Asia/Tel_Aviv
RUN ln -snf "/usr/share/zoneinfo/$TZ" /etc/localtime && \
    echo "$TZ" > /etc/timezone

RUN apt-get update && \
    apt-get install -y openssh-server sudo perl acl python3-apt && \
    sed -i -e 's:#PubkeyAuthentication.*:PubkeyAuthentication yes:g' /etc/ssh/sshd_config && \
    sed -i -e 's:#PasswordAuthentication.*:PasswordAuthentication no:g' /etc/ssh/sshd_config && \
    useradd -G "sudo" -m -p "$(perl -e 'print crypt("sshtest", "xD")')" sshtest && \
    useradd -G "sudo,sshtest" -m -p "$(perl -e 'print crypt("become", "xD")')" become && \
    sudo -u sshtest mkdir /home/sshtest/.ssh && \
    mkdir -p /run/sshd

COPY e2e/assets/ssh/id_rsa.pub /home/sshtest/.ssh/authorized_keys
COPY e2e/assets/ssh/id_rsa.pub /root/.ssh/authorized_keys

RUN sudo -u sshtest chmod 0700 /home/sshtest/.ssh && \
    chown sshtest:sshtest /home/sshtest/.ssh/authorized_keys && \
    sudo -u sshtest chmod 0600 /home/sshtest/.ssh/authorized_keys && \
    chmod 0600 /root/.ssh/authorized_keys

EXPOSE 22

CMD ["/usr/sbin/sshd", "-D", "-e"]