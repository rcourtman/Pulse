FROM golang:1.26.8@sha256:3c3e25a4da13fd0478eed2df1eb35a0e667094a7124d3993a6a1d30f71c17e79

# Set bash as default shell for features
SHELL ["/bin/bash", "-c"]

# Install dev tools needed for hot-dev.sh
RUN apt-get update && apt-get install -y     bash     git     make     curl     vim     sudo     lsof     inotify-tools     && rm -rf /var/lib/apt/lists/*

# Create non-root user
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN groupadd --gid $USER_GID $USERNAME &&     useradd --uid $USER_UID --gid $USER_GID -m $USERNAME -s /bin/bash &&     echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME &&     chmod 0440 /etc/sudoers.d/$USERNAME

# Set up Go environment for non-root user
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$PATH
RUN mkdir -p $GOPATH/pkg $GOPATH/bin &&     chown -R $USERNAME:$USERNAME $GOPATH

USER $USERNAME
WORKDIR /workspace

CMD ["/bin/bash"]
