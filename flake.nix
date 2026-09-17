{
  description = "Qovery Command Line Interface";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/22.05";
    utils.url = "github:numtide/flake-utils";
    flake-compat = {
      url = "github:edolstra/flake-compat";
      flake = false;
    };
  };

  outputs = { self, nixpkgs, utils, ... }:
    utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = import nixpkgs { inherit system; };
          name = "qovery-cli";
          version = "0.45.0"; # TODO: find a way to take the version from sources directly
          vendorSha256 = "KHLknBymDAwr7OxS2Ysx6WU5KQ9kmw0bE2Hlp3CBW0c=";

        in
        rec {
          # nix build
          defaultPackage = pkgs.buildGoModule rec {
            inherit version vendorSha256;
            pname = name;
            src = ./.;
          };

          # nix run
          defaultApp = utils.lib.mkApp {
            inherit name;
            drv = defaultPackage;
          };

          # nix develop
          devShell = pkgs.mkShell {
            inputsFrom = builtins.attrValues self.defaultPackage;
            nativeBuildInputs = with pkgs; [
              # Nix LSP + formatter
              rnix-lsp
              nixpkgs-fmt
            ];
          };
        }
      );
}
