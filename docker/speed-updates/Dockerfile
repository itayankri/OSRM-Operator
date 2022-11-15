FROM osrm/osrm-backend:latest

RUN apt update
RUN apt --assume-yes install curl

COPY run.sh run.sh
RUN chmod +x run.sh

ENTRYPOINT ["/run.sh"]