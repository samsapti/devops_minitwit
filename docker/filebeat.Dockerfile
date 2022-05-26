FROM docker.elastic.co/beats/filebeat-oss:7.10.2

COPY docker /docker
COPY setup-elk.sh /

WORKDIR /
USER 0

RUN sh /setup-elk.sh
RUN cp docker/filebeat/filebeat.yml /usr/share/filebeat/filebeat.yml

WORKDIR /usr/share/filebeat
USER 1000

ENTRYPOINT [ "/usr/local/bin/docker-entrypoint" ]
CMD [ "-environment", "container" ]

