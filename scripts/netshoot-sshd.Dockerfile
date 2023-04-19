FROM docker.io/nicolaka/netshoot:v0.8
RUN echo "https://mirrors.tuna.tsinghua.edu.cn/alpine/v3.16/main" > /etc/apk/repositories \
    && echo "https://mirrors.tuna.tsinghua.edu.cn/alpine/v3.16/community" >> /etc/apk/repositories \
    && apk add tini \
    && sed -i 's/AllowTcpForwarding no/AllowTcpForwarding yes/g' /etc/ssh/sshd_config \
    && sed -i 's/#PermitTunnel no/PermitTunnel yes/g' /etc/ssh/sshd_config  \
    && /usr/bin/ssh-keygen -A \
    && adduser -s /bin/zsh -D appuser \
    && rm -rf /var/cache/* && rm -rf /var/lib/apk/* && rm -rf /tmp/*
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/usr/sbin/sshd" , "-4eD"]
