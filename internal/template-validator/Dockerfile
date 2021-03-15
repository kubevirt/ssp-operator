FROM registry.access.redhat.com/ubi8/ubi-minimal
RUN mkdir -p /etc/webhook/certs

WORKDIR /
COPY ./_out/kubevirt-template-validator /usr/sbin/kubevirt-template-validator
USER 1000

ENTRYPOINT [ "/usr/sbin/kubevirt-template-validator" ]
