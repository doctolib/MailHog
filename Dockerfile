#
# MailHog Dockerfile
#

FROM golang:1.18-alpine as builder

# Install MailHog:
RUN apk --no-cache add --virtual build-dependencies \
    git \
  && mkdir -p /root/gocode \
  && export GOPATH=/root/gocode \
  && go install github.com/mailhog/MailHog@latest

FROM alpine:3
# Add mailhog user/group with uid/gid 1000.
# This is a workaround for boot2docker issue #581, see
# https://github.com/boot2docker/boot2docker/issues/581
RUN adduser -D -u 1000 mailhog

COPY --from=builder /root/gocode/bin/MailHog /usr/local/bin/

USER mailhog

COPY . ${BUILD_DIRECTORY}/MailHog
RUN find ${BUILD_DIRECTORY}/MailHog -exec ls -lshd '{}' +
RUN chown -R ${BUILD_USERNAME}:${BUILD_USERNAME} ${BUILD_DIRECTORY}/MailHog

# Build MailHog
USER ${BUILD_USERNAME}
WORKDIR ${BUILD_DIRECTORY}
ENV GOPATH="${BUILD_DIRECTORY}/go"
ENV PATH="$PATH:/go/bin:${GOPATH}/bin"
RUN mkdir -p go/{src,bin} bin
RUN make -C MailHog deps
RUN make -C MailHog
RUN mv MailHog/MailHog MailHog/cmd/mhsendmail/mhsendmail ${BUILD_DIRECTORY}/bin

FROM debian:bookworm-slim

# Create mailhog user as non-login system user with user-group
ARG USERNAME=mailhog
RUN useradd --shell /bin/false -Urb / -u 99 ${USERNAME}

# Copy mailhog binary
COPY --from=builder /home/build-user/bin/* /bin/

# Expose the SMTP and HTTP ports:
EXPOSE 2525 8025

WORKDIR /
USER ${USERNAME}
CMD ["MailHog"]
