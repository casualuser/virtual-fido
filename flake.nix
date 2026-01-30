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
        packages.virtual-fido = pkgs.buildGoModule {
          pname = "virtual-fido";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-RIHPXfK/fpnd1HZZZjAWQ7/vRaYACvTa+36lXVkDB48=";
          subPackages = [ "cmd/virtual-fido" ];
          nativeBuildInputs = [ pkgs.go ];
          doCheck = false;
          
          buildPhase = ''
             CGO_ENABLED=0 go build -o virtual-fido ./cmd/virtual-fido
          '';
          
          installPhase = ''
             mkdir -p $out/bin
             mv virtual-fido $out/bin/
          '';
        };

        packages.virtual-fido-vhid = pkgs.buildGoModule {
          pname = "virtual-fido-vhid";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-RIHPXfK/fpnd1HZZZjAWQ7/vRaYACvTa+36lXVkDB48=";
          tags = [ "hidvirtual" ];
          subPackages = [ "cmd/virtual-fido" ];
          
          nativeBuildInputs = [ pkgs.go ] ++ pkgs.lib.optionals pkgs.stdenv.isDarwin [
            pkgs.swift
          ];
          buildInputs = pkgs.lib.optionals pkgs.stdenv.isDarwin [
            pkgs.apple-sdk_15
          ];
          
          preBuild = pkgs.lib.optionalString pkgs.stdenv.isDarwin ''
            bash transport/vhid/HIDVirtualDevice/build.sh
            export CGO_LDFLAGS="-L$(pwd)/transport/vhid/output -lHIDVirtualDevice -lVirtualFIDODevice_swift"
            export CGO_CFLAGS="-I$(pwd)/transport/vhid/output/include"
          '';
          
          postInstall = ''
            mv $out/bin/virtual-fido $out/bin/virtual-fido-vhid
          '';
        };

        packages.default = self.packages.${system}.virtual-fido;

        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.go
            pkgs.just
            pkgs.rsync
            pkgs.python3
            pkgs.xxd
          ];
        };
      }
    );
}
