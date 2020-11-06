FROM golang:1.15-alpine AS build
WORKDIR /go/src/github.com/utilitywarehouse/kube-applier
COPY . /go/src/github.com/utilitywarehouse/kube-applier
RUN apk --no-cache -X https://uk.alpinelinux.org/alpine/edge/testing add git gcc musl-dev kubectl curl &&\
  os=$(go env GOOS) &&\
  arch=$(go env GOARCH) &&\
  curl -Ls https://go.kubebuilder.io/dl/2.3.1/${os}/${arch} | tar -xz -C /tmp/ &&\
  mv /tmp/kubebuilder_2.3.1_${os}_${arch} /usr/local/kubebuilder &&\
  export PATH=$PATH:/usr/local/kubebuilder/bin &&\
  go get -t ./... &&\
  CGO_ENABLED=1 && go test -race -count=1 ./... &&\
  CGO_ENABLED=0 && go build -o /kube-applier .

FROM alpine:3.12
ENV KUBECTL_VERSION v1.19.2
ENV KUSTOMIZE_VERSION v3.8.4
COPY templates/ /templates/
COPY static/ /static/
RUN apk --no-cache add git openssh-client tini &&\
  wget -O /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl &&\
  chmod +x /usr/local/bin/kubectl &&\
  wget -O - https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64.tar.gz |\
  tar xz -C /usr/local/bin/
COPY --from=build /kube-applier /kube-applier

ENTRYPOINT ["/sbin/tini", "--"]
CMD [ "/kube-applier" ]
