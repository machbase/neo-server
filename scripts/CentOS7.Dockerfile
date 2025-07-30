#################################################
## Build the image with:
# docker build -t centos7-build-env -f ./scripts/CentOS7.Dockerfile .
#
## Run the container with:
# docker run --rm -v ./tmp:/app/tmp -v ./packages:/app/packages centos7-build-env
#
#################################################

FROM centos:7

COPY ./scripts/CentOS7_repo.txt /etc/yum.repos.d/CentOS-Base.repo

RUN yum clean all && \
    yum makecache && \
    yum -y update && \
    yum -y group install "Development Tools" && \
    yum -y install wget tar gzip which && \
    echo "Install Go" && \
    wget -L https://go.dev/dl/go1.24.5.linux-amd64.tar.gz -O /tmp/go.tar.gz && \
    tar -C /usr/local -xzf /tmp/go.tar.gz && \
    rm -f /tmp/go.tar.gz && \
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /root/.bashrc

WORKDIR /app
COPY . /app

CMD ["/usr/local/go/bin/go", "run", "mage.go", "install-neo-web", "machbase-neo"]

