FROM osrm/osrm-backend:latest

RUN apt update
RUN apt --assume-yes install curl \
    jq

COPY run.sh run.sh
RUN chmod +x run.sh

CMD ["./run.sh"]