# syntax=docker/dockerfile:1.4
FROM golang:1.24-alpine AS build

ARG TARGETOS="linux" \
  TARGETARCH="arm64"
COPY . /src
WORKDIR /src

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/wrapper ./wrapper
#RUN --mount=target=. \
#    --mount=type=cache,target=/root/.cache/go-build \
#    --mount=type=cache,target=/go/pkg \
#    pwd && echo "----" && ls -la && \
#    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/wrapper ./wrapper

FROM debian:bookworm AS image

ARG DUPLICATI_RELEASE="v2.1.0.118_canary_2025-05-12" \
  TARGETPLATFORM="linux/arm64" \
  # Workaround for missing System.CommandLine (https://github.com/duplicati/duplicati/issues/6022)
  DOTNET_SCL_VERSION="2.0.0-beta4.22272.1"

## TODO! LABELS
# "io.hass.arch": "aarch64",
# "io.hass.description": "Zero-trust backup from any operating system to any destination that you can manage from anywhere.",
# "io.hass.name": "Duplicati",
# "io.hass.type": "addon",
# "io.hass.url": "https://github.com/echocat/hassos-addons/duplicati",
# "io.hass.version": "0.0.1"

ENV PATH="/opt/duplicati:${PATH}"

RUN \
    mkdir -p /config \
    && mkdir -p /data \
    && mkdir -p /backup \
    && if [ "$TARGETPLATFORM" = "linux/amd64" ]; then \
      VARIANT="linux-x64"; \
    elif [ "$TARGETPLATFORM" = "linux/arm64" ]; then \
      VARIANT="linux-arm64"; \
    elif [ "$TARGETPLATFORM" = "linux/arm/v7" ]; then \
      VARIANT="linux-arm7"; \
    else \
      echo "FATAL ERROR: Unsupported TARGETPLATFORM: $TARGETPLATFORM" \
      && exit 1; \
    fi \
    && echo -e "\n\n\e[1;94m+------------------+\n| Install Packages |\n+------------------+\e[0m" \
    && echo "deb http://deb.debian.org/debian bookworm contrib non-free" > /etc/apt/sources.list.d/contrib.list \
    && echo ttf-mscorefonts-installer msttcorefonts/accepted-mscorefonts-eula select true | debconf-set-selections \
    && apt-get update  \
    && apt-get install -y \
       tini \
       ca-certificates \
       bash \
       jq \
       curl \
       libicu72 \
       ttf-mscorefonts-installer \
       unzip \
       xz-utils \
    && echo -e "\n\n\e[1;94m+---------------------+\n| Resolve Environment |\n+---------------------+\e[0m" \
    && if [ -z ${DUPLICATI_RELEASE+x} ]; then \
        DUPLICATI_RELEASE=$(curl -sX GET "https://api.github.com/repos/duplicati/duplicati/releases/latest" | jq -r .tag_name); \
      fi \
    && echo -e "\e[1;94mUsing release: ${DUPLICATI_RELEASE}\e[0m" \
    && echo -e "\e[1;94mUsing variant: ${VARIANT}\e[0m" \
    && echo -e "\n\n\e[1;94m+--------------------+\n| Download Duplicati |\n+--------------------+\e[0m" \
    && download_url=$(curl -s "https://api.github.com/repos/duplicati/duplicati/releases/tags/${DUPLICATI_RELEASE}" | jq -r '.assets[].browser_download_url' |grep "$VARIANT-gui.zip\$") \
    && curl -o /tmp/duplicati.zip -L "${download_url}" \
    && echo -e "\n\n\e[1;94m+-------------------+\n| Install Duplicati |\n+-------------------+\e[0m" \
    && unzip -q /tmp/duplicati.zip -d /opt \
    && mv /opt/duplicati* /opt/duplicati \
    && echo -e "\n\n\e[1;94m+----------------------------+\n| Fix Duplicati dependencies |\n+----------------------------+\e[0m" \
    && if [ -n ${DOTNET_SCL_VERSION+x} ]; then \
        curl -o /tmp/dotnet_scl.zip -L "https://www.nuget.org/api/v2/package/System.CommandLine/${DOTNET_SCL_VERSION}" \
        && unzip -p -q /tmp/dotnet_scl.zip lib/netstandard2.0/System.CommandLine.dll > /opt/duplicati/System.CommandLine.dll \
      ; fi \
    && echo -e "\n\n\e[1;94m+---------+\n| Cleanup |\n+---------+\e[0m" \
    && apt-get clean \
    && rm -rf /tmp/* \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /var/tmp/* \
    && rm -rf /var/log/*

COPY --from=build /out/wrapper /opt/duplicati/wrapper

CMD [ "/opt/duplicati/wrapper" ]
