# To learn more about how to use Nix to configure your environment
# see: https://developers.google.com/idx/guides/customize-idx-env
{ pkgs, ... }: {
  # Which nixpkgs channel to use.
  channel = "unstable";
  # Use https://search.nixos.org/packages to find packages
  packages = [
    pkgs.docker
    pkgs.getopt
    pkgs.gnumake
    pkgs.go
    pkgs.golangci-lint
    pkgs.mockgen
    pkgs.nodejs_18
    pkgs.pnpm
    pkgs.shellcheck.bin
    pkgs.shfmt
    pkgs.sqlc
    pkgs.terraform
  ];

  # Sets environment variables in the workspace
  env = {
    GOPRIVATE = "coder.com,cdr.dev,go.coder.com,github.com/cdr,github.com/coder";
    NODE_OPTIONS = "--max-old-space-size=8192";
  };

  # enable docker
  services.docker.enable = true;

  idx = {
    # Search for the extensions you want on https://open-vsx.org/ and use "publisher.id"
    extensions = [
      "dbaeumer.vscode-eslint"
      "EditorConfig.EditorConfig"
      "emeraldwalk.RunOnSave"
      "esbenp.prettier-vscode"
      "foxundermoon.shell-format"
      "GitHub.vscode-codeql"
      "golang.go"
      "hashicorp.terraform"
      "redhat.vscode-yaml"
      "streetsidesoftware.code-spell-checker"
      "zxh404.vscode-proto3"
    ];

    # Enable previews
    previews = {
      enable = true;
      previews = {
        web = {
          # Example: run "npm run dev" with PORT set to IDX's defined port for previews,
          # and show it in IDX's web preview panel
          command = ["./scripts/develop.sh"];
          manager = "web";
          env = {
            # Environment variables to set for your server
            CODER_DEV_ACCESS_URL = "http://127.0.0.1:$PORT";
          };
        };
      };
    };

    # Workspace lifecycle hooks
    workspace = {
      # Runs when a workspace is first created
      onCreate = {
        # install dependencies
        pnpm-install = "cd site && pnpm install";
      };
      # Runs when the workspace is (re)started
      onStart = {
        # Example: start a background task to watch and re-build backend code
        # watch-backend = "npm run watch-backend";
      };
    };
  };
}
