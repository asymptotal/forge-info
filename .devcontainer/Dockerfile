FROM mcr.microsoft.com/devcontainers/go:1.27-trixie

USER vscode
RUN curl -fsSL https://claude.ai/install.sh | bash && \
    echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc && \
    echo "alias bruce='claude --dangerously-skip-permissions'" >> ~/.bashrc
