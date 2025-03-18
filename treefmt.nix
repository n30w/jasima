{
  # See https://github.com/numtide/treefmt-nix#supported-programs

  projectRootFile = "flake.nix";

  settings.global.excludes = [
    ".gitignore"
    ".gitattributes"
    "*.svg"
    "*.png"
    "*.jpg"
    "*.example"
    ".envrc"
    ".vscode/*"
    "outputs/*"
    "resources/*"
  ];

  programs.gofumpt.enable = true;
  programs.protolint.enable = true;
  programs.sqlfluff = {
    enable = true;
    dialect = "postgres";
  };

  # GitHub Actions
  programs.yamlfmt.enable = true;
  programs.actionlint.enable = true;

  programs.taplo.enable = true;

  # Markdown
  programs.mdformat.enable = true;

  # Nix
  programs.nixfmt.enable = true;
}
