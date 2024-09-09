FROM itayankri/osrm-builder:osrm-v5.27.1

RUN apt update
RUN apt --assume-yes install curl

COPY run.sh run.sh
RUN chmod +x run.sh

CMD ["./run.sh"]