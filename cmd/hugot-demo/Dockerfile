FROM golang:1.7-wheezy

ADD dist/prometheus /go/bin/prometheus
ADD dist/alertmanager /go/bin/alertmanager
RUN mkdir /conf
ADD prometheus.yml /conf/prometheus.yml
ADD alertmanager.yml /conf/alertmanager.yml

ADD docker_run.sh /usr/local/bin/docker_run.sh
CMD /usr/local/bin/docker_run.sh

ADD dist/hugot-demo /go/bin/hugot-demo

