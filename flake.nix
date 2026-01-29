{
  description = "Seed-based virtual FIDO toolchain";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.virtual-fido-vhid = pkgs.buildGoModule {
          pname = "virtual-fido-vhid";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-5+9zJ8Y9IuKzGzI3LzL6zI3LzL6zI3LzL6zI3LzL6zI="; # Placeholder
          proxyVendor = true;
          tags = [ "hidvirtual" ];
          subPackages = [ "cmd/virtual-fido" ];
          
          nativeBuildInputs = [ pkgs.go ];
          buildInputs = pkgs.lib.optionals pkgs.stdenv.isDarwin [
            pkgs.darwin.apple_sdk.frameworks.CoreFoundation
            pkgs.darwin.apple_sdk.frameworks.IOKit
            pkgs.darwin.apple_sdk.frameworks.AppKit
          ];
          
          postInstall = ''
            mv $out/bin/virtual-fido $out/bin/virtual-fido-vhid
          '';
        };

        packages.default = self.packages.${system}.virtual-fido-vhid;

        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.go
            pkgs.just
            pkgs.rsync
            pkgs.python3
            pkgs.xxd
          ] ++ pkgs.lib.optionals pkgs.stdenv.isDarwin [
            pkgs.darwin.apple_sdk.frameworks.CoreFoundation
            pkgs.darwin.apple_sdk.frameworks.IOKit
            pkgs.darwin.apple_sdk.frameworks.AppKit
          ];
        };
      }
    );
}
