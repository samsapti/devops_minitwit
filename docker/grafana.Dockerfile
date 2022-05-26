FROM grafana/grafana-oss:latest

USER 0

RUN rm -rf /etc/grafana/provisioning /var/lib/grafana/dashboards \
    && mkdir -p /etc/grafana/ /var/lib/grafana

USER 472

COPY docker/grafana/provisioning /etc/grafana/
COPY docker/grafana/dashboards /var/lib/grafana/

ENTRYPOINT [ "/run.sh" ]