FROM scratch

COPY log2fluent /

ENTRYPOINT ["/log2fluent"]
