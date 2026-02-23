{
  description = "Monorepo for standalone Waybar module backends";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forAllSystems =
        f:
        nixpkgs.lib.genAttrs systems (
          system:
          f {
            inherit system;
            pkgs = import nixpkgs { inherit system; };
          }
        );

      moduleDefs = {
        "agent-usage" = {
          src = ./modules/agent-usage;
          bin = "waybar-agent-usage";
          vendorHash = null;
        };

        github = {
          src = ./modules/github;
          bin = "waybar-github";
          vendorHash = "sha256-SGZah7Hz7cGLk/8cOKkMpA4eGEtRCyENCPAjJQeswsc=";
        };

        linear = {
          src = ./modules/linear;
          bin = "waybar-linear";
          vendorHash = "sha256-SGZah7Hz7cGLk/8cOKkMpA4eGEtRCyENCPAjJQeswsc=";
        };

        schedule = {
          src = ./modules/schedule;
          bin = "waybar-schedule";
          vendorHash = "sha256-v42cUuT2z+y7BKjVTG/cbIQc32HG2YQ7k/zjNfPsL6s=";
        };

        sotto = {
          src = ./modules/sotto;
          bin = "waybar-sotto";
          vendorHash = "sha256-RZ5TFzYh/bVICuQqvVfcU+o9BCe9brMV0rpRlAjNpE8=";
        };
      };
    in
    {
      packages = forAllSystems (
        { pkgs, ... }:
        let
          builtPackages = nixpkgs.lib.mapAttrs (
            _: def:
            pkgs.buildGoModule {
              pname = def.bin;
              version = "0.1.0";
              src = def.src;
              env.GOWORK = "off";
              vendorHash = def.vendorHash;
              subPackages = [ "cmd/${def.bin}" ];
              ldflags = [
                "-s"
                "-w"
              ];
            }
          ) moduleDefs;

          waybar-modules = pkgs.symlinkJoin {
            name = "waybar-modules-0.1.0";
            paths = builtins.attrValues builtPackages;
          };
        in
        builtPackages
        // {
          inherit waybar-modules;
          default = waybar-modules;
        }
      );

      apps = forAllSystems (
        { system, ... }:
        let
          moduleApps = nixpkgs.lib.mapAttrs (
            name: def: {
              type = "app";
              program = "${self.packages.${system}.${name}}/bin/${def.bin}";
            }
          ) moduleDefs;
        in
        moduleApps
        // {
          default = moduleApps.linear;
        }
      );

      devShells = forAllSystems (
        { pkgs, ... }:
        {
          default = pkgs.mkShell {
            nativeBuildInputs = with pkgs; [
              go
              golangci-lint
              just
              pre-commit
            ];
          };
        }
      );
    };
}
