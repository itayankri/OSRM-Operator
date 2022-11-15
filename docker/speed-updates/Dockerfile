FROM osrm/osrm-backend:latest

RUN apt update
RUN apt --assume-yes install curl

COPY fetch.sh fetch.sh
RUN chmod +x fetch.sh

ENTRYPOINT ["/run.sh"]