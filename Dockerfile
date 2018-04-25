FROM golang:1.9.4
RUN apt-get update -y && apt-get install -y strace