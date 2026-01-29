let
  flake = import (fetchTarball {
    url = "https://github.com/edolstra/flake-compat/archive/master.tar.gz";
    sha256 = "1m9v69hgp6vsqn2f6v90117yqsyxgy3l7sn29w2300lmdlzh2j52";
  }) {
    src =  ./.;
  };
in flake.shellNix
