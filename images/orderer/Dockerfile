FROM ubuntu_with_software:latest as builder

# 进行 go 环境变量的配置
# ----------------------------------------------------------------
ENV GOLANG_VERSION=1.23.2
ENV GOPROXY=https://mirrors.aliyun.com/goproxy/
COPY ./images/orderer/resources/go${GOLANG_VERSION}.linux-amd64.tar.gz /usr/local/

RUN cd /usr/local && \
    tar -xzf go${GOLANG_VERSION}.linux-amd64.tar.gz && \
    rm go${GOLANG_VERSION}.linux-amd64.tar.gz

ENV PATH="$PATH:/usr/local/go/bin"
ENV GOPATH="/go"
ENV PATH="$PATH:$GOPATH/bin"
# ----------------------------------------------------------------

# 进行编译
# ----------------------------------------------------------------
ADD . .
RUN make orderer GO_TAGS="" FABRIC_VER=3.0.0
RUN make ccaasbuilder
# ----------------------------------------------------------------

# 运行时镜像
# ----------------------------------------------------------------
FROM ubuntu_with_software:latest
RUN echo 'hosts: files dns' > /etc/nsswitch.conf

ENV     FABRIC_CFG_PATH /etc/hyperledger/fabric
ENV     FABRIC_VER      3.0.0

COPY    --from=builder  build/bin/orderer           /usr/local/bin
COPY    --from=builder  sampleconfig/msp            ${FABRIC_CFG_PATH}/msp
COPY    --from=builder  sampleconfig/orderer.yaml   ${FABRIC_CFG_PATH}
COPY    --from=builder  sampleconfig/configtx.yaml  ${FABRIC_CFG_PATH}
COPY    ./images/orderer/resources/daemons /etc/frr/daemons


VOLUME  /etc/hyperledger/fabric
VOLUME  /var/hyperledger

EXPOSE 7050

CMD     [ "orderer", "start" ]
# ----------------------------------------------------------------