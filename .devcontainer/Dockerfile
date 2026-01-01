FROM golang:1.24-bookworm

# Install Node.js 20
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - &&     apt-get install -y nodejs &&     npm install -g npm@latest

# Install additional dev tools
RUN apt-get update && apt-get install -y     git     make     docker.io     curl     vim     && rm -rf /var/lib/apt/lists/*

# Create non-root user
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN groupadd --gid $USER_GID $USERNAME &&     useradd --uid $USER_UID --gid $USER_GID -m $USERNAME &&     apt-get update &&     apt-get install -y sudo &&     echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME &&     chmod 0440 /etc/sudoers.d/$USERNAME

# Set up Go environment for non-root user
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$PATH
RUN mkdir -p $GOPATH/pkg $GOPATH/bin &&     chown -R $USERNAME:$USERNAME $GOPATH

USER $USERNAME
WORKDIR /workspace

# Install common Go tools
RUN go install golang.org/x/tools/gopls@latest &&     go install github.com/go-delve/delve/cmd/dlv@latest

CMD ["/bin/bash"]
