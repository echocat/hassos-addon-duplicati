# syntax=docker/dockerfile:1.4
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

ARG TARGETOS \
  TARGETARCH

WORKDIR /src
COPY . .

RUN --mount=target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/wrapper ./wrapper

FROM debian:bookworm AS image

ARG DUPLICATI_RELEASE \
  TARGETPLATFORM \
  # Workaround for missing System.CommandLine (https://github.com/duplicati/duplicati/issues/6022)
  DOTNET_SCL_VERSION="2.0.0-beta4.22272.1"

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
       ca-certificates \
       bash \
       jq \
       curl \
       libicu72 \
       ttf-mscorefonts-installer \
       unzip \
       xz-utils \
    && echo -e "\n\n\e[1;94m+---------------------+\n| Resolve Environment |\n+---------------------+\e[0m" \
    && echo -e "\e[1;94mUsing release: ${DUPLICATI_RELEASE}\e[0m" \
    && echo -e "\e[1;94mUsing variant: ${VARIANT}\e[0m" \
    && echo -e "\n\n\e[1;94m+--------------------+\n| Download Duplicati |\n+--------------------+\e[0m" \
    && download_url=$(curl -s "https://api.github.com/repos/duplicati/duplicati/releases/tags/${DUPLICATI_RELEASE}" | jq -r '.assets[].browser_download_url' |grep "$VARIANT-gui.zip\$") \
    && curl -o /tmp/duplicati.zip -L "${download_url}" \
    && echo -e "\n\n\e[1;94m+-------------------+\n| Install Duplicati |\n+-------------------+\e[0m" \
    && unzip -q /tmp/duplicati.zip -d /opt \
    && mv /opt/duplicati* /opt/duplicati \
    && echo -e "\n\n\e[1;94m+----------------------------+\n| Fix Duplicati dependencies |\n+----------------------------+\e[0m" \
    && if [ -n ${DOTNET_SCL_VERSION+x} ] && [ ! -f /opt/duplicati/System.CommandLine.dll ]; then \
        echo "Add /opt/duplicati/System.CommandLine.dll ..." \
        && curl -o /tmp/dotnet_scl.zip -L "https://www.nuget.org/api/v2/package/System.CommandLine/${DOTNET_SCL_VERSION}" \
        && unzip -p -q /tmp/dotnet_scl.zip lib/netstandard2.0/System.CommandLine.dll > /opt/duplicati/System.CommandLine.dll \
      ; fi \
    && echo -e "\n\n\e[1;94m+---------+\n| Cleanup |\n+---------+\e[0m" \
    && apt-get remove -y \
       jq \
       unzip \
       xz-utils \
    && apt-get clean \
    && rm -rf /tmp/* \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /var/tmp/* \
    && rm -rf /var/log/*

COPY --from=build /out/wrapper /opt/duplicati/wrapper

CMD [ "/opt/duplicati/wrapper" ]
