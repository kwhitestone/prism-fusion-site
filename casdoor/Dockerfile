ARG REGISTRY=swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io

FROM --platform=$BUILDPLATFORM ${REGISTRY}/node:22.22.0-alpine3.23 AS FRONT
WORKDIR /web_prism

# Use China npm mirror
RUN npm config set registry https://registry.npmmirror.com

# Copy only dependency files first for better caching
COPY ./web_prism/package.json ./web_prism/yarn.lock ./
RUN yarn config set registry https://registry.npmmirror.com && \
    yarn install --frozen-lockfile --network-timeout 1000000

# Copy source files and build
COPY ./web_prism .
RUN NODE_OPTIONS="--max-old-space-size=4096" yarn run build

FROM --platform=$BUILDPLATFORM ${REGISTRY}/library/golang:1.24.6 AS BACK
WORKDIR /go/src/casdoor

# Use China Go module mirror
ENV GOPROXY=https://goproxy.cn,direct

# Copy only go.mod and go.sum first for dependency caching
COPY ./casdoor/go.mod ./casdoor/go.sum ./
RUN go mod download

# Copy source files
COPY ./casdoor .

# Skip TestGetVersionInfo - requires .git which is excluded by .dockerignore
RUN ./build.sh

FROM ${REGISTRY}/alpine:latest AS STANDARD
LABEL MAINTAINER="https://casdoor.org/"
ARG USER=casdoor
ARG TARGETOS
ARG TARGETARCH
ENV BUILDX_ARCH="${TARGETOS:-linux}_${TARGETARCH:-amd64}"

RUN sed -i 's/https/http/' /etc/apk/repositories
RUN apk add --update sudo
RUN apk add tzdata
RUN apk add curl
RUN apk add ca-certificates && update-ca-certificates

RUN adduser -D $USER -u 1000 \
    && echo "$USER ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/$USER \
    && chmod 0440 /etc/sudoers.d/$USER \
    && mkdir logs \
    && chown -R $USER:$USER logs

USER 1000
WORKDIR /
COPY --from=BACK --chown=$USER:$USER /go/src/casdoor/server_${BUILDX_ARCH} ./server
COPY --from=BACK --chown=$USER:$USER /go/src/casdoor/swagger ./swagger
COPY --chown=$USER:$USER ./casdoor_conf/app.conf ./conf/app.conf
COPY --from=FRONT --chown=$USER:$USER /web_prism/build ./web/build

ENTRYPOINT ["/server"]
