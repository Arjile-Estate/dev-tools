{ pkgs, ... }:

{
  languages.go.enable = true;

  packages = with pkgs; [
    codex
    docker-client
    golangci-lint
  ];

  scripts.dev-tools.exec = ''
    exec go run . "$@"
  '';

  enterTest = ''
    dev-tools test
  '';
}
