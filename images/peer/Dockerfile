FROM ubuntu_with_software:latest as builder

# 进行 go 环境变量的配置
# ----------------------------------------------------------------
ENV GOLANG_VERSION=1.23.2

COPY ./resources/go${GOLANG_VERSION}.linux-amd64.tar.gz /usr/local/

RUN cd /usr/local && \
    tar -xzf go${GOLANG_VERSION}.linux-amd64.tar.gz && \
    rm go${GOLANG_VERSION}.linux-amd64.tar.gz

ENV PATH="$PATH:/usr/local/go/bin"
ENV GOPATH="/go"
ENV PATH="$PATH:$GOPATH/bin"
# ----------------------------------------------------------------

# 进行编译
# ----------------------------------------------------------------
ARG GO_TAGS
ADD . .
RUN make peer GO_TAGS=${GO_TAGS} FABRIC_VER=3.0.0
RUN make ccaasbuilder
# ----------------------------------------------------------------

# 运行时镜像
# ----------------------------------------------------------------
FROM ubuntu_with_software:latest
RUN echo 'hosts: files dns' > /etc/nsswitch.conf

ENV     FABRIC_CFG_PATH /etc/hyperledger/fabric
ENV     FABRIC_VER      3.0.0

COPY    --from=builder  build/bin/peer          /usr/local/bin
COPY    --from=builder  sampleconfig/msp        ${FABRIC_CFG_PATH}/msp
COPY    --from=builder  sampleconfig/core.yaml  ${FABRIC_CFG_PATH}/core.yaml

COPY    --from=builder  release/linux-amd64/builders/ccaas/bin /opt/hyperledger/ccaas_builder/bin

VOLUME  /etc/hyperledger/fabric
VOLUME  /var/hyperledger

EXPOSE 7051

CMD    [ "peer", "node", "start" ]
# ----------------------------------------------------------------