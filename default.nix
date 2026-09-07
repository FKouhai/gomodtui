{
  pkgs,
  go,
}:
let
  pname = "gomodtui";
in
pkgs.buildGoApplication {
  inherit go pname;

  version = "0.1.0";
  src = ./.;

  meta.mainProgram = pname;
}
