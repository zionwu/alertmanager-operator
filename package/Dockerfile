FROM ubuntu:16.04
RUN apt-get update && apt-get install --no-install-recommends -y curl ca-certificates
COPY alertmanager-operator /usr/bin/
CMD ["alertmanager-operator"]
