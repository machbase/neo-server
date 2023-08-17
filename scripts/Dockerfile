FROM ubuntu:22.04 AS build-stage
ARG TARGETARCH

COPY . .

RUN if [ "$TARGETARCH" = "amd64" ]; then \
        cp amd64/machbase-neo /opt/machbase-neo; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
        cp arm64/machbase-neo /opt/machbase-neo; \
    fi

FROM ubuntu:22.04 AS runtime-stage 

LABEL MAINTAINER="machbase.com <support@machbase.com>"

COPY --from=build-stage /opt/machbase-neo /opt/machbase-neo

EXPOSE 5652-5656

VOLUME ["/data"]

ENTRYPOINT /opt/machbase-neo serve \
           --host 0.0.0.0 \
           --data /data

