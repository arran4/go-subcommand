FROM scratch
COPY gosubc /
USER 1001
ENTRYPOINT ["/gosubc"]
