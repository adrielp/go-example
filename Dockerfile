FROM scratch

ARG BIN_PATH=go-example

ARG UID=10001
USER ${UID}

COPY --chmod=755 ${BIN_PATH} /usr/bin/go-example


ENTRYPOINT ["/usr/bin/go-example"]
