FROM ubuntu:22.04 

LABEL MAINTAINER="machbase.com <support@machbase.com>"

COPY machbase-neo /opt/machbase-neo

EXPOSE 5652-5656

VOLUME ["/data"]

ENTRYPOINT /opt/machbase-neo serve \
           --host 0.0.0.0 \
           --data /data

