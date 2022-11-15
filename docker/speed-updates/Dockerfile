FROM osrm/osrm-backend:latest

RUN apt update
RUN apt --assume-yes install curl

COPY entrypoint.sh entrypoint.sh
RUN chmod +x entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]